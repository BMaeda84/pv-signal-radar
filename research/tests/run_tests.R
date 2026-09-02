#!/usr/bin/env Rscript

file_arg <- grep("^--file=", commandArgs(trailingOnly = FALSE), value = TRUE)
script_path <- if (length(file_arg) == 1L) sub("^--file=", "", file_arg) else "research/tests/run_tests.R"
research_root <- dirname(dirname(normalizePath(script_path, mustWork = FALSE)))
Sys.setenv(TZ = "UTC")
suppressWarnings(Sys.setlocale("LC_COLLATE", "C"))
source(file.path(research_root, "R", "pipeline.R"), local = TRUE)
source(file.path(research_root, "R", "reference_metrics.R"), local = TRUE)

tests_run <- 0L
expect_true <- function(value, message) {
  tests_run <<- tests_run + 1L
  if (!isTRUE(value)) stop(sprintf("FAIL: %s", message), call. = FALSE)
}
expect_equal <- function(actual, expected, tolerance = .Machine$double.eps^0.5, message = "values differ") {
  tests_run <<- tests_run + 1L
  if (!isTRUE(all.equal(actual, expected, tolerance = tolerance, check.attributes = FALSE))) {
    stop(sprintf("FAIL: %s\nExpected: %s\nActual: %s", message, paste(expected, collapse = ", "), paste(actual, collapse = ", ")), call. = FALSE)
  }
}
expect_error <- function(expression, pattern, message) {
  tests_run <<- tests_run + 1L
  error <- tryCatch({ force(expression); NULL }, error = identity)
  if (is.null(error) || !grepl(pattern, conditionMessage(error))) stop(sprintf("FAIL: %s", message), call. = FALSE)
}

fixture <- file.path(research_root, "tests", "fixtures", "ASCII")

# Different source headers that collapse to one normalized name are ambiguous:
# accepting either column would make schema drift silently change the ETL.
header_collision_fixtures <- list(
  demo = c("PRIMARYID$CASEID$CASEVERSION$CASE-ID$CASE ID", "1$1$1$x$y"),
  drug = c("PRIMARYID$ROLE_COD$DRUGNAME$ROLE COD", "1$PS$DRUG A$SS"),
  reac = c("PRIMARYID$CASE-ID$CASE ID$PT", "1$x$y$NAUSEA")
)
for (table_name in names(header_collision_fixtures)) {
  collision_path <- tempfile(fileext = ".txt")
  writeLines(header_collision_fixtures[[table_name]], collision_path, useBytes = TRUE)
  expect_error(
    read_faers_ascii(collision_path, table_name),
    "header columns collide after normalization",
    sprintf("%s header-name collisions must fail closed", toupper(table_name))
  )
  unlink(collision_path)
}

demo <- read_faers_ascii(file.path(fixture, "DEMO24Q1.txt"), "demo")
drug <- read_faers_ascii(file.path(fixture, "DRUG24Q1.txt"), "drug")
reac <- read_faers_ascii(file.path(fixture, "REAC24Q1.txt"), "reac")
demo$quarter <- "2024Q1"

current <- deduplicate_cases(demo)
integrity <- validate_relational_integrity(demo, drug, reac)
expect_true(all(integrity == 0), "fixture joins must have no orphan or CASEID-mismatched child rows")
expect_equal(nrow(current), 4L, message = "latest case-version deduplication must retain four current case reports")
expect_true("1002" %in% as.character(current$primaryid), "new CASEVERSION must supersede the older PRIMARYID")
expect_true(!"1001" %in% as.character(current$primaryid), "superseded PRIMARYID must not enter report pairs")
large_ids <- demo[1:2, , drop = FALSE]
large_ids$caseid <- "900719925474099312345"
large_ids$caseversion <- c("900719925474099312345", "900719925474099312346")
large_ids$primaryid <- c("900719925474099312347", "900719925474099312348")
expect_equal(deduplicate_cases(large_ids)$primaryid, "900719925474099312348", message = "case/version ordering must preserve identifiers beyond double precision")
leading_zero_demo <- demo
leading_zero_drug <- drug
leading_zero_reac <- reac
leading_zero_demo$primaryid[[1]] <- "001001"
leading_zero_drug$primaryid[leading_zero_drug$primaryid == "1001"] <- "001001"
leading_zero_reac$primaryid[leading_zero_reac$primaryid == "1001"] <- "001001"
expect_true(all(validate_relational_integrity(leading_zero_demo, leading_zero_drug, leading_zero_reac) == 0), "leading-zero identifiers must canonicalize consistently across relational tables")
tie_demo <- demo[1:2, , drop = FALSE]
tie_demo$caseversion <- "2"
tie_current <- deduplicate_cases(tie_demo)
expect_equal(as.character(tie_current$primaryid), "1002", message = "PRIMARYID must deterministically break a same-CASEVERSION tie")

pairs <- build_report_pairs(current, drug, reac)
expect_equal(nrow(pairs), 5L, message = "duplicate PT rows must collapse at report-drug-role-event level")
expect_equal(sum(pairs$primaryid == "1002" & pairs$event_pt == "NAUSEA"), 1L, message = "duplicate NAUSEA entry must be unique")
expect_true(all(pairs$event_category == "UNCLASSIFIED_SOURCE_PT"), "pipeline must not infer a clinical category")
report_counts <- summarize_report_counts(demo, current, pairs)
expect_equal(
  unname(report_counts[c("source_demo_rows", "current_case_reports", "eligible_reports", "drug_event_pairs")]),
  c(5, 4, 3, 5),
  message = "QA must distinguish raw DEMO rows, retained current reports, eligible reports, and unique report pairs"
)
expect_true(
  report_counts[["source_demo_rows"]] > report_counts[["current_case_reports"]],
  "a superseded fixture row must remain visible only in the source DEMO count"
)
eligible_demo <- current[as.character(current$primaryid) %in% unique(as.character(pairs$primaryid)), , drop = FALSE]
field_completeness <- build_field_completeness(eligible_demo, c("age", "sex", "occr_country", "serious"))
expect_true(
  all(vapply(field_completeness, function(field) field$population == "eligible_reports" && field$denominator_records == 3L, logical(1))),
  "field missingness must declare the eligible-report population and its denominator"
)
expect_true(
  all(vapply(field_completeness, function(field) field$missing_records == 0L, logical(1))),
  "missing demographics on an ineligible current report must not contaminate eligible-report missingness"
)
superseded_pair <- pairs[1, , drop = FALSE]
superseded_pair$primaryid <- "1001"
expect_error(
  summarize_report_counts(demo, current, rbind(pairs, superseded_pair)),
  "not a retained current case version",
  "QA must reject an eligible pair derived from a superseded report version"
)
duplicated_drug <- rbind(drug, drug, drug[1, , drop = FALSE])
duplicated_reac <- rbind(reac, reac, reac[1, , drop = FALSE])
expect_equal(
  build_report_pairs(current, duplicated_drug, duplicated_reac),
  pairs,
  message = "report-level DRUG and REAC deduplication before the join must preserve pair semantics"
)
duplicate_component_qa <- attr(build_report_pairs(current, duplicated_drug, duplicated_reac), "component_qa")
expect_true(
  duplicate_component_qa[["duplicate_drug_components_removed"]] > 0 &&
    duplicate_component_qa[["duplicate_reaction_components_removed"]] > 0,
  "QA must report pre-join duplicate components removed without changing pair semantics"
)
explosive_drug <- drug[drug$primaryid == "1002", , drop = FALSE][rep(1L, 101L), , drop = FALSE]
explosive_drug$prod_ai <- sprintf("DISTINCT DRUG %03d", seq_len(nrow(explosive_drug)))
explosive_reac <- reac[reac$primaryid == "1002", , drop = FALSE][rep(1L, 101L), , drop = FALSE]
explosive_reac$pt <- sprintf("DISTINCT EVENT %03d", seq_len(nrow(explosive_reac)))
expect_error(
  build_report_pairs(current, explosive_drug, explosive_reac),
  "join budget exceeded",
  "distinct per-report drugs times events must be budgeted before the Cartesian merge"
)

cells <- build_aggregate(pairs, "fixture-2024q1")
golden_cells <- utils::read.delim(
  file.path(research_root, "tests", "golden", "aggregate_interchange.tsv"),
  sep = "\t", quote = "", check.names = FALSE, stringsAsFactors = FALSE
)
expect_equal(cells, golden_cells, message = "R aggregate must remain byte-contract compatible with the Go materializer golden fixture")
serialized_cells <- tempfile(fileext = ".tsv")
on.exit(unlink(serialized_cells), add = TRUE)
utils::write.table(cells, serialized_cells, sep = "\t", quote = FALSE, row.names = FALSE, col.names = TRUE, eol = "\n", fileEncoding = "UTF-8")
read_raw <- function(path) readBin(path, what = "raw", n = file.info(path)$size)
expect_true(identical(read_raw(serialized_cells), read_raw(file.path(research_root, "tests", "golden", "aggregate_interchange.tsv"))), "R TSV serialization bytes must match the manifest-bound Go golden artifact")
aspirin_ps_nausea <- cells[cells$drug_text == "ASPIRIN" & cells$drug_role == "primary_suspect" & cells$event_pt == "NAUSEA", ]
expect_equal(unlist(aspirin_ps_nausea[, c("a", "b", "c", "d")]), c(1, 1, 1, 0), message = "fixture 2x2 cells must match the hand-calculated table")
expect_true(all(c("primary_suspect", "secondary_suspect", "suspect", "all") %in% unique(cells$drug_role)), "roles present in the fixture and pre-specified unions must use the API contract")
expect_equal(map_faers_drug_role(c("PS", "SS", "C", "I")), c("primary_suspect", "secondary_suspect", "concomitant", "interacting"), message = "every documented FDA source role must have an explicit API mapping")
expect_true(all(cells$comparator == "all_other_eligible_reports"), "aggregate comparator must match the research API contract")
expect_true(all(cells$event_scope == "all_recorded_source_pts"), "aggregate event scope must match the research API contract")
expect_true(all(cells$a + cells$b + cells$c + cells$d == cells$universe_reports), "every aggregate row must reconcile to its universe")
expect_equal(cells, build_aggregate(pairs, "fixture-2024q1"), message = "aggregate transformation must be deterministic")

metrics <- calculate_reference_metrics(1, 1, 1, 0)
expect_true(metrics$zero_correction_applied, "zero cells must record Haldane-Anscombe correction")
expect_equal(metrics$prr, 2 / 3, tolerance = 1e-12, message = "corrected PRR must match the hand calculation")
expect_equal(metrics$ror, 1 / 3, tolerance = 1e-12, message = "corrected ROR must match the hand calculation")
expect_equal(metrics$prr_lower_95, 0.16673176883993762, tolerance = 1e-12, message = "corrected PRR lower CI must match pvda/Go qnorm(0.975)")
expect_equal(metrics$prr_upper_95, 2.6656254386115865, tolerance = 1e-12, message = "corrected PRR upper CI must match pvda/Go qnorm(0.975)")
expect_equal(metrics$ror_lower_95, 0.006614174656049977, tolerance = 1e-12, message = "corrected ROR lower CI must match pvda/Go qnorm(0.975)")
expect_equal(metrics$ror_upper_95, 16.798938172804046, tolerance = 1e-12, message = "corrected ROR upper CI must match pvda/Go qnorm(0.975)")
expect_true(metrics$fisher_two_sided_p >= 0 && metrics$fisher_two_sided_p <= 1, "Fisher p-value must be bounded")
regular_metrics <- calculate_reference_metrics(10, 90, 20, 880)
expect_equal(regular_metrics$prr, 4.5, tolerance = 1e-12, message = "uncorrected PRR must match the hand calculation")
expect_equal(adjust_fdr(c(0.01, 0.04, 0.5)), c(0.03, 0.06, 0.5), tolerance = 1e-12, message = "BH adjustment must match the reference values")

invalid_demo <- demo
invalid_demo$caseversion[[1]] <- "not-a-number"
expect_error(deduplicate_cases(invalid_demo), "invalid positive decimal", "invalid case keys must fail closed")
invalid_register <- data.frame(
  quarter = "2024Q1", file = "faers.zip", source_url = "https://www.fda.gov/example",
  retrieved_at = "2024-04-01T00:00:00Z", coverage_start = "2024-01-01", coverage_end = "2024-03-31", sha256 = "",
  stringsAsFactors = FALSE
)
expect_error(validate_source_register_metadata(invalid_register), "SHA-256", "blank source hashes must be rejected")
expect_error(validate_archive_entries(c("ascii/DEMO24Q1.txt", "../escape.txt")), "unsafe", "ZIP path traversal must be rejected")
archive_limits <- archive_extraction_limits()
expect_error(
  validate_archive_budget(
    data.frame(Name = sprintf("member-%03d.txt", seq_len(archive_limits$max_members + 1L)), Length = 1),
    archive_bytes = archive_limits$max_members + 1L
  ),
  "member budget",
  "ZIP archives exceeding the explicit member budget must be rejected before extraction"
)
expect_error(
  validate_archive_budget(
    data.frame(Name = "oversized.txt", Length = archive_limits$max_uncompressed_bytes + 1),
    archive_bytes = archive_limits$max_uncompressed_bytes + 1
  ),
  "uncompressed-size budget",
  "ZIP archives exceeding the uncompressed byte budget must be rejected before extraction"
)
zip_bomb_path <- tempfile(fileext = ".zip")
zip_bomb_payload <- tempfile(pattern = "small-zip-bomb-", fileext = ".txt")
on.exit(unlink(c(zip_bomb_path, zip_bomb_payload)), add = TRUE)
if (nzchar(Sys.which("zip"))) {
  # One MiB of repeated bytes is deliberately small in absolute terms but
  # compresses enough to exercise the same central-directory ratio guard used
  # before extraction. The committed suite has no large binary bomb fixture.
  writeBin(raw(1024^2), zip_bomb_payload)
  utils::zip(zip_bomb_path, files = zip_bomb_payload, flags = "-j -q")
  zip_bomb_listing <- utils::unzip(zip_bomb_path, list = TRUE)
  zip_bomb_bytes <- file.info(zip_bomb_path)$size
} else {
  # Metadata-equivalent fallback keeps the base-R suite portable on hosts that
  # lack a zip executable; the pinned research container executes the real ZIP.
  zip_bomb_listing <- data.frame(Name = "small-zip-bomb.txt", Length = 1024^2)
  zip_bomb_bytes <- 512
}
expect_error(
  validate_archive_budget(zip_bomb_listing, archive_bytes = zip_bomb_bytes),
  "expansion-ratio budget",
  "a small ZIP bomb must be rejected before extraction"
)
expect_error(
  validate_faers_table_quarters(
    c("DEMO24Q1.txt", "DRUG24Q2.txt", "REAC24Q1.txt"),
    "2024Q1"
  ),
  "does not match table filename",
  "a table mislabeled with another quarter must be rejected"
)
wrong_coverage_register <- invalid_register
wrong_coverage_register$sha256 <- strrep("a", 64L)
wrong_coverage_register$coverage_end <- "2024-04-01"
expect_error(
  validate_source_register_metadata(wrong_coverage_register),
  "exactly match",
  "source coverage must exactly match the declared calendar quarter"
)
untrusted_register <- invalid_register
untrusted_register$sha256 <- strrep("a", 64L)
untrusted_register$source_url <- "https://www.fda.gov.attacker.example/faers.zip"
expect_error(
  validate_source_register_metadata(untrusted_register),
  "official HTTPS",
  "formal FAERS source metadata must reject lookalike non-FDA hosts"
)
expect_error(
  validate_tsv_formula_safety(data.frame(event_pt = "=HYPERLINK(\"https://example.invalid\")")),
  "formula trigger",
  "TSV fields capable of spreadsheet formula execution must fail closed"
)
complete_commit <- strrep("a", 40L)
baked_commit_file <- tempfile(pattern = "pv-radar-baked-commit-")
writeLines(complete_commit, baked_commit_file, useBytes = TRUE)
expect_error(
  verify_pipeline_commit("e4223c5", research_root, runtime_commit = complete_commit, git_executable = "", baked_commit_path = baked_commit_file),
  "complete lowercase",
  "abbreviated commits must not identify scientific builds"
)
test_tampered_runtime_environment <- function() {
  previous_commit <- Sys.getenv("PV_RADAR_PIPELINE_COMMIT", unset = NA_character_)
  on.exit({
    if (is.na(previous_commit)) {
      Sys.unsetenv("PV_RADAR_PIPELINE_COMMIT")
    } else {
      Sys.setenv(PV_RADAR_PIPELINE_COMMIT = previous_commit)
    }
  }, add = TRUE)
  Sys.setenv(PV_RADAR_PIPELINE_COMMIT = strrep("b", 40L))
  expect_error(
    verify_pipeline_commit(complete_commit, research_root, git_executable = "", baked_commit_path = baked_commit_file),
    "differs from PV_RADAR_PIPELINE_COMMIT",
    "a tampered runtime environment must not override the baked container revision"
  )
}
test_tampered_runtime_environment()
expect_equal(
  verify_pipeline_commit(complete_commit, research_root, runtime_commit = complete_commit, git_executable = "", baked_commit_path = baked_commit_file),
  complete_commit,
  message = "a complete commit matching baked and runtime provenance must be accepted"
)
expect_error(
  verify_pipeline_commit(complete_commit, research_root, runtime_commit = complete_commit, git_executable = "", baked_commit_path = paste0(baked_commit_file, "-missing")),
  "Baked pipeline commit is required",
  "a matching runtime environment must not substitute for baked provenance"
)
expect_error(
  verify_pipeline_commit(strrep("b", 40L), research_root, runtime_commit = strrep("b", 40L), git_executable = "", baked_commit_path = baked_commit_file),
  "differs from the baked pipeline commit",
  "matching requested and runtime revisions must still be rejected when the image was baked from another commit"
)
unlink(baked_commit_file)
expect_error(map_faers_drug_role("NEW_ROLE"), "unsupported ROLE_COD", "unknown FDA role codes must fail closed")
orphan_drug <- drug
orphan_drug$primaryid[[1]] <- "999999"
expect_error(validate_relational_integrity(demo, orphan_drug, reac), "orphan", "orphan DRUG rows must fail closed")
mismatched_reac <- reac
mismatched_reac$caseid[[1]] <- "999999"
expect_error(validate_relational_integrity(demo, drug, mismatched_reac), "mismatch", "PRIMARYID/CASEID disagreement must fail closed")

cat(sprintf("PASS: %d research fixture assertions\n", tests_run))
