#!/usr/bin/env Rscript

`%||%` <- function(left, right) if (!is.null(left)) left else right
file_arg <- grep("^--file=", commandArgs(trailingOnly = FALSE), value = TRUE)
script_path <- if (length(file_arg) == 1L) sub("^--file=", "", file_arg) else "research/R/build_snapshot.R"
script_path <- normalizePath(script_path, mustWork = FALSE)
research_root <- dirname(dirname(script_path))
source(file.path(research_root, "R", "pipeline.R"), local = TRUE)

options(error = function() {
  traceback(2L)
  quit(status = 1L)
})

named <- parse_named_arguments(commandArgs(trailingOnly = TRUE))
required_args <- c("source-register", "output", "dataset-id", "build-timestamp", "software-version")
missing_args <- required_args[!required_args %in% names(named)]
if (length(missing_args) > 0L) stop(sprintf("Missing arguments: %s", paste(missing_args, collapse = ", ")), call. = FALSE)
for (package in c("arrow", "data.table", "digest", "jsonlite")) {
  if (!requireNamespace(package, quietly = TRUE)) stop(sprintf("Package '%s' is required; run research/R/bootstrap.R", package), call. = FALSE)
}

# Locale-sensitive case conversion and sorting are part of the transformation,
# so a scientific build fixes and records them instead of inheriting a host
# default. C.UTF-8 preserves Unicode case conversion while C collation provides
# stable byte-oriented ordering in the controlled Linux build environment.
requested_locale <- named$locale %||% "C.UTF-8"
active_ctype <- suppressWarnings(Sys.setlocale("LC_CTYPE", requested_locale))
active_collate <- suppressWarnings(Sys.setlocale("LC_COLLATE", "C"))
active_numeric <- suppressWarnings(Sys.setlocale("LC_NUMERIC", "C"))
if (!nzchar(active_ctype) || !nzchar(active_collate) || !nzchar(active_numeric)) {
  stop(sprintf("Required locale is unavailable: LC_CTYPE=%s, LC_COLLATE=C, LC_NUMERIC=C", requested_locale), call. = FALSE)
}
Sys.setenv(TZ = "UTC")
source_encoding <- named$encoding %||% "UTF-8"

lock_path <- named$`renv-lock` %||% file.path(research_root, "renv.lock")
lock_path <- normalizePath(lock_path, mustWork = TRUE)
lock <- jsonlite::read_json(lock_path, simplifyVector = FALSE)
required_locked_packages <- c("arrow", "data.table", "digest", "jsonlite", "faers", "pvda", "openEBGM", "renv")
missing_locked <- setdiff(required_locked_packages, names(lock$Packages))
if (length(missing_locked) > 0L) {
  stop(sprintf("renv.lock is missing required package(s): %s", paste(missing_locked, collapse = ", ")), call. = FALSE)
}
locked_package_names <- sort(names(lock$Packages))
for (package in locked_package_names) {
  actual_version_object <- tryCatch(utils::packageVersion(package), error = function(error) NULL)
  if (is.null(actual_version_object)) stop(sprintf("Locked package '%s' is not installed", package), call. = FALSE)
  actual_version <- as.character(actual_version_object)
  locked_version <- as.character(lock$Packages[[package]]$Version)
  # R normalizes package version separators (for example 1.90.0-1 is reported
  # as 1.90.0.1), so compare parsed versions while retaining both source forms.
  if (!isTRUE(actual_version_object == base::package_version(locked_version))) {
    stop(sprintf("Package '%s' differs from renv.lock: installed=%s locked=%s", package, actual_version, locked_version), call. = FALSE)
  }
}
if (!is.null(lock$R$Version) && !identical(as.character(getRversion()), as.character(lock$R$Version))) {
  stop(sprintf("R version differs from renv.lock: running=%s locked=%s", getRversion(), lock$R$Version), call. = FALSE)
}
if (!grepl("^[a-z0-9]([a-z0-9._-]{0,126}[a-z0-9])?$", named$`dataset-id`)) stop("dataset-id must match the research API lowercase path-safe contract", call. = FALSE)
named$`software-version` <- verify_pipeline_commit(named$`software-version`, research_root)
if (!grepl("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$", named$`build-timestamp`)) {
  stop("build-timestamp must be an explicit UTC timestamp (YYYY-MM-DDTHH:MM:SSZ)", call. = FALSE)
}
if (is.na(as.POSIXct(named$`build-timestamp`, format = "%Y-%m-%dT%H:%M:%SZ", tz = "UTC"))) stop("build-timestamp is not a valid UTC date", call. = FALSE)

register_path <- normalizePath(named$`source-register`, mustWork = TRUE)
output_target <- normalizePath(named$output, mustWork = FALSE)
if (file.exists(output_target) || dir.exists(output_target)) stop("Refusing to overwrite an existing output path", call. = FALSE)
dir.create(dirname(output_target), recursive = TRUE, showWarnings = FALSE)
# Build beside the target and rename only after every artifact and checksum is
# complete. A failed process therefore cannot expose a partial dataset as ready.
output <- tempfile(pattern = paste0(".", basename(output_target), ".staging-"), tmpdir = dirname(output_target))
dir.create(output, recursive = TRUE, showWarnings = FALSE)
on.exit(if (dir.exists(output)) unlink(output, recursive = TRUE, force = TRUE), add = TRUE)

register <- utils::read.csv(register_path, stringsAsFactors = FALSE, check.names = FALSE)
validate_source_register_metadata(register)
register$.resolved_path <- vapply(register$file, function(path) {
  candidate <- if (grepl("^[A-Za-z]:|^/", path)) path else file.path(dirname(register_path), path)
  normalizePath(candidate, mustWork = TRUE)
}, character(1))
actual_hash <- vapply(register$.resolved_path, digest::digest, character(1), algo = "sha256", file = TRUE, serialize = FALSE)
if (any(tolower(actual_hash) != tolower(register$sha256))) {
  bad <- register$file[tolower(actual_hash) != tolower(register$sha256)]
  stop(sprintf("Source checksum mismatch: %s", paste(bad, collapse = ", ")), call. = FALSE)
}

work <- tempfile("pv-radar-faers-")
dir.create(work)
on.exit(unlink(work, recursive = TRUE, force = TRUE), add = TRUE)
demo_parts <- drug_parts <- reac_parts <- list()
quarters <- sort(unique(register$quarter))
for (quarter in quarters) {
  tables <- materialize_quarter_tables(register[register$quarter == quarter, , drop = FALSE], work)
  demo <- read_faers_ascii(tables$demo, "demo", source_encoding)
  drug <- read_faers_ascii(tables$drug, "drug", source_encoding)
  reac <- read_faers_ascii(tables$reac, "reac", source_encoding)
  demo$quarter <- quarter
  drug$quarter <- quarter
  reac$quarter <- quarter
  demo_parts[[quarter]] <- demo
  drug_parts[[quarter]] <- drug
  reac_parts[[quarter]] <- reac
}

demo_all <- do.call(rbind, demo_parts)
drug_all <- do.call(rbind, drug_parts)
reac_all <- do.call(rbind, reac_parts)
relational_qa <- validate_relational_integrity(demo_all, drug_all, reac_all)
current_demo <- deduplicate_cases(demo_all)
pairs <- build_report_pairs(current_demo, drug_all, reac_all)
component_qa <- attr(pairs, "component_qa")
if (is.null(component_qa)) stop("Internal error: report component QA was not produced", call. = FALSE)
report_counts <- summarize_report_counts(demo_all, current_demo, pairs)
aggregate <- build_aggregate(pairs, named$`dataset-id`)
duplicate_caseversion_rows <- sum(duplicated(demo_all[, c("caseid", "caseversion"), drop = FALSE]))

pairs_path <- file.path(output, "report_pairs")
arrow::write_dataset(
  pairs,
  pairs_path,
  format = "parquet",
  partitioning = "quarter",
  existing_data_behavior = "error"
)
aggregate_path <- file.path(output, "aggregate_interchange.tsv")
# normalize_text removes tabs/newlines from source text. Formula-triggering
# prefixes are rejected instead of escaped because the Go materializer consumes
# the exact source terms and no reversible cross-language encoding exists yet.
validate_tsv_formula_safety(aggregate)
data.table::fwrite(aggregate, aggregate_path, sep = "\t", quote = FALSE, na = "", eol = "\n")

qa <- data.frame(
  metric = c(
    "source_quarters", "source_demo_rows", "current_case_reports", "superseded_demo_rows", "duplicate_caseversion_rows",
    "drug_rows_raw", "reaction_rows_raw", "eligible_reports", "drug_event_pairs",
    "distinct_drug_role_groups", "distinct_event_pts", "aggregate_rows",
    names(relational_qa), names(component_qa),
    "drug_rows_for_superseded_reports", "reaction_rows_for_superseded_reports"
  ),
  value = c(
    length(quarters), report_counts[["source_demo_rows"]], report_counts[["current_case_reports"]],
    report_counts[["source_demo_rows"]] - report_counts[["current_case_reports"]], duplicate_caseversion_rows,
    nrow(drug_all), nrow(reac_all), report_counts[["eligible_reports"]], report_counts[["drug_event_pairs"]],
    nrow(unique(pairs[, c("drug_text", "drug_text_source", "drug_role")])),
    length(unique(pairs$event_pt)), nrow(aggregate),
    unname(relational_qa), unname(component_qa),
    sum(!as.character(drug_all$primaryid) %in% as.character(current_demo$primaryid)),
    sum(!as.character(reac_all$primaryid) %in% as.character(current_demo$primaryid))
  ),
  stringsAsFactors = FALSE
)
qa_path <- file.path(output, "qa_summary.csv")
utils::write.csv(qa, qa_path, row.names = FALSE, na = "", fileEncoding = "UTF-8")

published_register <- register[, c("quarter", "file", "source_url", "retrieved_at", "coverage_start", "coverage_end", "sha256")]
published_register$file <- basename(published_register$file)
published_register <- published_register[order(published_register$quarter, published_register$file), , drop = FALSE]
source_manifest_path <- file.path(output, "source_manifest.csv")
utils::write.csv(published_register, source_manifest_path, row.names = FALSE, na = "", fileEncoding = "UTF-8")

package_versions <- vapply(locked_package_names, function(package) as.character(utils::packageVersion(package)), character(1))
environment <- list(
  schema_version = "pv-signal-radar.environment/v1",
  build_timestamp = named$`build-timestamp`,
  source_commit = named$`software-version`,
  r_version = R.version.string,
  r_platform = R.version$platform,
  source_encoding = source_encoding,
  locale = list(ctype = active_ctype, collate = active_collate, numeric = active_numeric),
  timezone = Sys.getenv("TZ"),
  renv_lock_sha256 = digest::digest(lock_path, algo = "sha256", file = TRUE, serialize = FALSE),
  package_versions = as.list(package_versions)
)
dir.create(file.path(output, "metadata"), recursive = TRUE, showWarnings = FALSE)
environment_path <- file.path(output, "metadata", "environment.json")
jsonlite::write_json(environment, environment_path, auto_unbox = TRUE, pretty = TRUE, na = "null")
published_lock_path <- file.path(output, "metadata", "renv.lock")
if (!file.copy(lock_path, published_lock_path, overwrite = FALSE, copy.mode = TRUE, copy.date = FALSE)) {
  stop("Could not bind renv.lock into the dataset artifact", call. = FALSE)
}

artifact_files <- sort(c(
  list.files(pairs_path, recursive = TRUE, full.names = TRUE),
  aggregate_path, qa_path, source_manifest_path, environment_path, published_lock_path
))
artifact_relative <- vapply(artifact_files, function(path) {
  substring(normalizePath(path, winslash = "/"), nchar(normalizePath(output, winslash = "/")) + 2L)
}, character(1))
artifact_hashes <- vapply(artifact_files, digest::digest, character(1), algo = "sha256", file = TRUE, serialize = FALSE)
artifact_media_type <- function(path) {
  extension <- tolower(tools::file_ext(path))
  switch(extension, parquet = "application/vnd.apache.parquet", tsv = "text/tab-separated-values", csv = "text/csv", json = "application/json", lock = "application/json", "application/octet-stream")
}
artifacts <- lapply(seq_along(artifact_files), function(index) list(
  name = artifact_relative[[index]],
  path = artifact_relative[[index]],
  media_type = artifact_media_type(artifact_files[[index]]),
  sha256 = artifact_hashes[[index]],
  bytes = as.numeric(file.info(artifact_files[[index]])$size)
))

eligible_demo <- current_demo[as.character(current_demo$primaryid) %in% unique(as.character(pairs$primaryid)), , drop = FALSE]
if (nrow(eligible_demo) != report_counts[["eligible_reports"]]) {
  stop("Eligible DEMO denominator does not match the unique report-pair population", call. = FALSE)
}
completeness_fields <- build_field_completeness(eligible_demo, c("age", "sex", "occr_country", "serious"))
source_files <- lapply(seq_len(nrow(register)), function(index) list(
  url = register$source_url[[index]],
  sha256 = tolower(register$sha256[[index]]),
  bytes = as.numeric(file.info(register$.resolved_path[[index]])$size)
))
manifest <- list(
  schema_version = "pv-signal-radar.research/v1",
  dataset_id = named$`dataset-id`,
  title = sprintf("FDA FAERS frozen research snapshot %s", paste(quarters, collapse = ", ")),
  description = "Academic pharmacovigilance research and education; not causal, clinical, GxP, or regulatory decision support",
  source = list(
    name = "FDA Adverse Event Reporting System (FAERS) quarterly ASCII files",
    publisher = "U.S. Food and Drug Administration",
    landing_page = "https://www.fda.gov/drugs/fda-adverse-event-monitoring-system-aems/fda-adverse-event-monitoring-system-aems-latest-quarterly-data-files",
    retrieved_at = max(register$retrieved_at),
    license = "Verify FDA source and redistribution terms for the selected release",
    files = source_files
  ),
  coverage = list(
    start_date = min(register$coverage_start),
    end_date = max(register$coverage_end),
    geography = "Global reports as recorded in FAERS source fields",
    release = paste(quarters, collapse = ",")
  ),
  processing = list(
    pipeline_version = "pv-signal-radar.faers-etl/v1",
    source_commit = named$`software-version`,
    deduplication_policy = "MAX_CASEVERSION_PER_CASEID_THEN_MAX_PRIMARYID",
    count_unit = "Unique PRIMARYID x source drug text x role x source PT; marginals count unique PRIMARYID",
    drug_role_policy = "Map FDA PS/SS/C/I to primary_suspect/secondary_suspect/concomitant/interacting; derive deduplicated suspect and all unions",
    exclusions = c("Superseded case versions", "Rows without a current report, non-empty drug text and role, or non-empty PT")
  ),
  vocabularies = list(
    list(name = "MedDRA Preferred Term text from FAERS", version = "UNSPECIFIED_BY_QUARTERLY_SOURCE", scope = "Event PT source text; no inferred hierarchy", license = "MedDRA terms and licenses may apply"),
    list(name = "FAERS PROD_AI/DRUGNAME source text", version = paste(quarters, collapse = ","), scope = "PROD_AI preferred, otherwise DRUGNAME; no concept mapping", license = "FDA source terms apply")
  ),
  artifacts = artifacts,
  completeness = list(
    source_demo_rows = report_counts[["source_demo_rows"]],
    current_case_reports = report_counts[["current_case_reports"]],
    eligible_reports = report_counts[["eligible_reports"]],
    drug_event_pairs = report_counts[["drug_event_pairs"]],
    fields = completeness_fields
  ),
  limitations = c(
    "No exposure denominator or incidence estimate",
    "No causal attribution or clinical adjudication",
    "No inferred MedDRA version or canonical drug concept",
    "Eligible analysis universe excludes current reports without both a retained drug row and PT"
  )
)
manifest_path <- file.path(output, "manifest.json")
jsonlite::write_json(manifest, manifest_path, auto_unbox = TRUE, pretty = TRUE, na = "null")

files_to_hash <- sort(c(
  artifact_files,
  manifest_path
))
hashes <- vapply(files_to_hash, digest::digest, character(1), algo = "sha256", file = TRUE, serialize = FALSE)
relative <- vapply(files_to_hash, function(path) {
  substring(normalizePath(path, winslash = "/"), nchar(normalizePath(output, winslash = "/")) + 2L)
}, character(1))
checksum_lines <- sprintf("%s  %s", hashes, relative)
writeLines(checksum_lines, file.path(output, "checksums.sha256"), useBytes = TRUE)
if (file.exists(output_target) || dir.exists(output_target) || !file.rename(output, output_target)) {
  stop("Artifacts are complete but atomic publication to the requested output path failed", call. = FALSE)
}
message(sprintf("Built dataset %s with %d eligible report(s) and %d aggregate row(s)", named$`dataset-id`, length(unique(pairs$primaryid)), nrow(aggregate)))
