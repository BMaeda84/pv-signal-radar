# Deterministic FAERS transformation helpers.
#
# The core transformations deliberately use source identifiers and source PT text.
# Concept normalization is a separate, versioned activity: silently guessing an
# ingredient or MedDRA mapping would make a result look more precise than its data.

normalize_column_names <- function(x) {
  tolower(gsub("(^_+|_+$)", "", gsub("[^A-Za-z0-9]+", "_", trimws(x))))
}

normalize_unique_column_names <- function(x, table_name) {
  normalized <- normalize_column_names(x)
  collisions <- unique(normalized[duplicated(normalized)])
  if (length(collisions) > 0L) {
    stop(
      sprintf(
        "%s header columns collide after normalization: %s",
        toupper(table_name),
        paste(collisions, collapse = ", ")
      ),
      call. = FALSE
    )
  }
  normalized
}

normalize_text <- function(x) {
  x <- enc2utf8(as.character(x))
  x <- gsub("[[:space:]]+", " ", trimws(x))
  toupper(x)
}

validate_full_commit <- function(commit, field_name = "software-version") {
  commit <- trimws(as.character(commit))
  if (length(commit) != 1L || is.na(commit) ||
      !grepl("^([a-f0-9]{40}|[a-f0-9]{64})$", commit)) {
    stop(sprintf("%s must be a complete lowercase 40- or 64-character Git commit", field_name), call. = FALSE)
  }
  commit
}

read_baked_pipeline_commit <- function(path) {
  path <- trimws(as.character(path))
  if (length(path) != 1L || is.na(path) || !nzchar(path) || !file.exists(path)) return("")
  if (dir.exists(path)) stop("Baked pipeline commit path must be a regular file", call. = FALSE)

  lines <- readLines(path, warn = FALSE)
  if (length(lines) != 1L) {
    stop("Baked pipeline commit file must contain exactly one revision", call. = FALSE)
  }
  validate_full_commit(lines[[1]], "baked pipeline commit")
}

verify_pipeline_commit <- function(requested_commit, research_root,
                                   runtime_commit = Sys.getenv("PV_RADAR_PIPELINE_COMMIT", unset = ""),
                                   git_executable = Sys.which("git"),
                                   baked_commit_path = "/usr/local/share/pv-signal-radar/pipeline-commit") {
  requested_commit <- validate_full_commit(requested_commit, "software-version")
  runtime_commit <- trimws(as.character(runtime_commit))
  if (length(runtime_commit) != 1L || is.na(runtime_commit)) {
    stop("PV_RADAR_PIPELINE_COMMIT must be a single value", call. = FALSE)
  }
  if (nzchar(runtime_commit)) {
    runtime_commit <- validate_full_commit(runtime_commit, "PV_RADAR_PIPELINE_COMMIT")
    if (!identical(requested_commit, runtime_commit)) {
      stop("software-version differs from PV_RADAR_PIPELINE_COMMIT", call. = FALSE)
    }
  }
  baked_commit <- read_baked_pipeline_commit(baked_commit_path)
  if (nzchar(baked_commit) && !identical(requested_commit, baked_commit)) {
    stop("software-version differs from the baked pipeline commit", call. = FALSE)
  }

  # A clean Git checkout is authoritative for local runs. The container omits
  # .git, so it must carry the root-owned commit file baked at image build. The
  # runtime environment may corroborate either source but can never replace it.
  repository_parent <- normalizePath(file.path(research_root, ".."), mustWork = TRUE)
  git_output <- function(arguments) {
    output <- suppressWarnings(system2(
      git_executable,
      c("-C", shQuote(repository_parent), arguments),
      stdout = TRUE,
      stderr = TRUE
    ))
    status <- attr(output, "status")
    if (is.null(status)) status <- 0L
    list(output = output, status = status)
  }
  has_git_checkout <- nzchar(git_executable)
  if (has_git_checkout) {
    probe <- git_output(c("rev-parse", "--is-inside-work-tree"))
    has_git_checkout <- identical(probe$status, 0L) && identical(trimws(tail(probe$output, 1L)), "true")
  }

  if (has_git_checkout) {
    head <- git_output(c("rev-parse", "HEAD"))
    if (!identical(head$status, 0L)) stop("Could not read the pipeline Git commit", call. = FALSE)
    checkout_commit <- validate_full_commit(trimws(tail(head$output, 1L)), "checkout Git commit")
    if (!identical(requested_commit, checkout_commit)) {
      stop("software-version differs from the checked-out Git commit", call. = FALSE)
    }
    status <- git_output(c("status", "--porcelain=v1", "--untracked-files=normal"))
    if (!identical(status$status, 0L)) stop("Could not inspect pipeline Git working tree", call. = FALSE)
    if (length(status$output) > 0L && any(nzchar(status$output))) {
      stop("Scientific snapshot requires a clean Git working tree", call. = FALSE)
    }
  } else if (!nzchar(baked_commit)) {
    stop("Baked pipeline commit is required when Git checkout metadata is unavailable", call. = FALSE)
  }

  requested_commit
}

map_faers_drug_role <- function(x) {
  source_role <- normalize_text(x)
  role_map <- c(
    PS = "primary_suspect",
    SS = "secondary_suspect",
    C = "concomitant",
    I = "interacting"
  )
  unknown <- !is.na(source_role) & nzchar(source_role) & !source_role %in% names(role_map)
  if (any(unknown)) {
    # Failing closed avoids silently folding a new FDA code into a scientifically
    # different role. Updating this mapping requires a versioned protocol change.
    stop(sprintf("DRUG contains unsupported ROLE_COD value(s): %s", paste(sort(unique(source_role[unknown])), collapse = ", ")), call. = FALSE)
  }
  unname(role_map[source_role])
}

assert_columns <- function(data, required, table_name) {
  missing <- setdiff(required, names(data))
  if (length(missing) > 0L) {
    stop(sprintf("%s is missing required columns: %s", table_name, paste(missing, collapse = ", ")), call. = FALSE)
  }
}

add_optional_columns <- function(data, names) {
  for (name in setdiff(names, colnames(data))) {
    data[[name]] <- NA_character_
  }
  data
}

canonical_positive_decimal <- function(values, field_name) {
  values <- trimws(as.character(values))
  valid <- !is.na(values) & grepl("^[0-9]+$", values)
  normalized <- sub("^0+", "", values)
  normalized[normalized == ""] <- "0"
  valid <- valid & normalized != "0"
  if (any(!valid)) {
    stop(sprintf("%s contains %d invalid positive decimal identifier(s)", field_name, sum(!valid)), call. = FALSE)
  }
  normalized
}

read_faers_ascii <- function(path, table_name, encoding = "UTF-8") {
  wanted <- switch(
    table_name,
    demo = c(
      "primaryid", "caseid", "caseversion", "event_dt", "fda_dt", "age", "age_cod", "sex",
      "occr_country", "reporter_country", "serious", "seriousnessdeath",
      "seriousnesslifethreatening", "seriousnesshospitalization", "seriousnessdisabling",
      "seriousnesscongenitalanomali", "seriousnessother"
    ),
    drug = c("primaryid", "caseid", "role_cod", "drugname", "prod_ai", "route", "dose_vbm"),
    reac = c("primaryid", "caseid", "pt"),
    stop(sprintf("Unsupported FAERS table type: %s", table_name), call. = FALSE)
  )

  if (requireNamespace("data.table", quietly = TRUE)) {
    header <- data.table::fread(
      path,
      sep = "$",
      quote = "",
      nrows = 0L,
      encoding = encoding,
      showProgress = FALSE
    )
    original_names <- names(header)
    # FAERS column spellings vary across quarters. Normalization is convenient,
    # but two distinct source headers must never collapse to one semantic field;
    # otherwise selection could silently consume an arbitrary column.
    normalized_names <- normalize_unique_column_names(original_names, table_name)
    selected <- original_names[normalized_names %in% wanted]
    data <- data.table::fread(
      path,
      sep = "$",
      quote = "",
      select = selected,
      na.strings = c("", "NA"),
      encoding = encoding,
      colClasses = "character",
      showProgress = FALSE,
      data.table = FALSE
    )
  } else {
    # This fallback keeps the fixture suite runnable on a plain R installation.
    # Official quarters are large; production runs should install data.table.
    data <- utils::read.delim(
      path,
      sep = "$",
      quote = "",
      comment.char = "",
      na.strings = c("", "NA"),
      check.names = FALSE,
      stringsAsFactors = FALSE,
      fileEncoding = encoding,
      colClasses = "character"
    )
    normalized_names <- normalize_unique_column_names(names(data), table_name)
    data <- data[, normalized_names %in% wanted, drop = FALSE]
  }

  names(data) <- normalize_unique_column_names(names(data), table_name)
  required <- switch(
    table_name,
    demo = c("primaryid", "caseid", "caseversion"),
    drug = c("primaryid", "role_cod", "drugname"),
    reac = c("primaryid", "pt")
  )
  assert_columns(data, required, toupper(table_name))
  data <- add_optional_columns(data, wanted)
  data[, wanted, drop = FALSE]
}

deduplicate_cases <- function(demo) {
  assert_columns(demo, c("primaryid", "caseid", "caseversion", "quarter"), "DEMO")
  decimal_rank <- function(values) {
    unique_values <- unique(values)
    ordered_values <- unique_values[order(nchar(unique_values), unique_values, method = "radix")]
    match(values, ordered_values)
  }
  primary_key <- canonical_positive_decimal(demo$primaryid, "DEMO PRIMARYID")
  case_key <- canonical_positive_decimal(demo$caseid, "DEMO CASEID")
  version_key <- canonical_positive_decimal(demo$caseversion, "DEMO CASEVERSION")

  # Decimal-string ranks preserve exact identifiers beyond 2^53. Ranking by
  # digit count and then radix order is equivalent to integer ordering for
  # canonical positive decimal strings without converting to double.
  demo$primaryid <- primary_key
  demo$caseid <- case_key
  demo$caseversion <- version_key
  demo$.primary_rank <- decimal_rank(primary_key)
  demo$.case_rank <- decimal_rank(case_key)
  demo$.version_rank <- decimal_rank(version_key)
  # FDA case revisions supersede earlier versions. A primaryid tie-break makes
  # the policy total and deterministic while retaining a QA count for review.
  ordered <- order(demo$.case_rank, -demo$.version_rank, -demo$.primary_rank, demo$quarter, method = "radix")
  demo <- demo[ordered, , drop = FALSE]
  current <- demo[!duplicated(demo$.case_rank), , drop = FALSE]
  current <- current[order(current$.primary_rank, method = "radix"), , drop = FALSE]
  current$.primary_rank <- NULL
  current$.case_rank <- NULL
  current$.version_rank <- NULL
  if (anyDuplicated(as.character(current$primaryid))) {
    stop("Current DEMO contains a PRIMARYID assigned to more than one CASEID", call. = FALSE)
  }
  rownames(current) <- NULL
  current
}

validate_relational_integrity <- function(demo, drug, reac) {
  assert_columns(demo, c("primaryid", "caseid"), "DEMO")
  assert_columns(drug, c("primaryid", "caseid"), "DRUG")
  assert_columns(reac, c("primaryid", "caseid"), "REAC")

  demo_primary <- canonical_positive_decimal(demo$primaryid, "DEMO PRIMARYID")
  demo_case <- canonical_positive_decimal(demo$caseid, "DEMO CASEID")
  if (anyDuplicated(demo_primary)) {
    # PRIMARYID identifies a report version and must be a one-to-one parent key.
    # CASEID is expected to repeat across versions, but PRIMARYID is not.
    stop("DEMO contains duplicate PRIMARYID parent keys", call. = FALSE)
  }

  inspect_child <- function(child, table_name) {
    child_primary <- canonical_positive_decimal(child$primaryid, paste(table_name, "PRIMARYID"))
    child_case <- canonical_positive_decimal(child$caseid, paste(table_name, "CASEID"))
    parent_index <- match(child_primary, demo_primary)
    orphan <- is.na(parent_index)
    mismatch <- !orphan & child_case != demo_case[parent_index]
    if (any(orphan) || any(mismatch)) {
      stop(sprintf(
        "%s relational integrity failed: %d orphan row(s), %d PRIMARYID/CASEID mismatch(es)",
        table_name, sum(orphan), sum(mismatch)
      ), call. = FALSE)
    }
    c(orphan_rows = sum(orphan), caseid_mismatches = sum(mismatch))
  }

  drug_metrics <- inspect_child(drug, "DRUG")
  reac_metrics <- inspect_child(reac, "REAC")
  c(
    demo_duplicate_primaryids = sum(duplicated(demo_primary)),
    drug_orphan_rows = drug_metrics[["orphan_rows"]],
    drug_caseid_mismatches = drug_metrics[["caseid_mismatches"]],
    reaction_orphan_rows = reac_metrics[["orphan_rows"]],
    reaction_caseid_mismatches = reac_metrics[["caseid_mismatches"]]
  )
}

report_join_limits <- function() {
  list(max_pairs_per_report = 10000)
}

validate_report_join_budget <- function(drug, reac, limits = report_join_limits()) {
  assert_columns(drug, "primaryid", "deduplicated DRUG")
  assert_columns(reac, "primaryid", "deduplicated REAC")
  drug_counts <- table(as.character(drug$primaryid), useNA = "no")
  reaction_counts <- table(as.character(reac$primaryid), useNA = "no")
  report_ids <- intersect(names(drug_counts), names(reaction_counts))
  if (length(report_ids) == 0L) {
    return(invisible(list(max_projected_pairs_per_report = 0, reports_with_pairs = 0)))
  }

  # Even after exact-row deduplication, a report with many distinct drugs and
  # events creates a Cartesian product. Preflight the product using compact
  # marginal counts so an anomalous report cannot allocate the merged table.
  projected_pairs <- as.numeric(drug_counts[report_ids]) * as.numeric(reaction_counts[report_ids])
  excessive <- projected_pairs > limits$max_pairs_per_report
  if (any(excessive)) {
    worst <- order(projected_pairs[excessive], decreasing = TRUE)[1L]
    bad_ids <- report_ids[excessive]
    stop(sprintf(
      "Report-level join budget exceeded: PRIMARYID %s projects %.0f pairs (limit %.0f)",
      bad_ids[[worst]],
      projected_pairs[excessive][[worst]],
      limits$max_pairs_per_report
    ), call. = FALSE)
  }
  invisible(list(
    max_projected_pairs_per_report = max(projected_pairs),
    reports_with_pairs = length(report_ids)
  ))
}

build_report_pairs <- function(current_demo, drug, reac) {
  demo_optional <- c(
    "event_dt", "fda_dt", "age", "age_cod", "sex", "occr_country", "reporter_country",
    "serious", "seriousnessdeath", "seriousnesslifethreatening",
    "seriousnesshospitalization", "seriousnessdisabling",
    "seriousnesscongenitalanomali", "seriousnessother"
  )
  current_demo <- add_optional_columns(current_demo, demo_optional)
  drug <- add_optional_columns(drug, c("prod_ai", "route", "dose_vbm"))
  assert_columns(current_demo, c("primaryid", "caseid", "quarter"), "current DEMO")
  assert_columns(drug, c("primaryid", "role_cod", "drugname"), "DRUG")
  assert_columns(reac, c("primaryid", "pt"), "REAC")

  current_demo$primaryid <- canonical_positive_decimal(current_demo$primaryid, "current DEMO PRIMARYID")
  current_demo$caseid <- canonical_positive_decimal(current_demo$caseid, "current DEMO CASEID")
  current_ids <- current_demo$primaryid
  drug$primaryid <- canonical_positive_decimal(drug$primaryid, "DRUG PRIMARYID")
  drug$caseid <- canonical_positive_decimal(drug$caseid, "DRUG CASEID")
  reac$primaryid <- canonical_positive_decimal(reac$primaryid, "REAC PRIMARYID")
  reac$caseid <- canonical_positive_decimal(reac$caseid, "REAC CASEID")
  drug <- drug[drug$primaryid %in% current_ids, , drop = FALSE]
  reac <- reac[reac$primaryid %in% current_ids, , drop = FALSE]

  ingredient <- normalize_text(drug$prod_ai)
  product <- normalize_text(drug$drugname)
  use_ingredient <- !is.na(ingredient) & nzchar(ingredient)
  drug$drug_text <- ifelse(use_ingredient, ingredient, product)
  drug$drug_text_source <- ifelse(use_ingredient, "PROD_AI", "DRUGNAME")
  drug$drug_role <- map_faers_drug_role(drug$role_cod)
  drug$route <- normalize_text(drug$route)
  drug$dose_vbm <- normalize_text(drug$dose_vbm)
  reac$event_pt <- normalize_text(reac$pt)

  drug <- drug[!is.na(drug$drug_text) & nzchar(drug$drug_text) & !is.na(drug$drug_role) & nzchar(drug$drug_role), , drop = FALSE]
  reac <- reac[!is.na(reac$event_pt) & nzchar(reac$event_pt), , drop = FALSE]

  # Source files may repeat identical DRUG or REAC rows. Removing those rows at
  # report level before the many-to-many merge preserves the final set of
  # report-drug-event pairs while preventing a quadratic intermediate table.
  drug_key <- c("primaryid", "drug_text", "drug_text_source", "drug_role")
  reaction_key <- c("primaryid", "event_pt")
  eligible_drug_component_rows <- nrow(drug)
  eligible_reaction_component_rows <- nrow(reac)
  drug <- drug[!duplicated(drug[, drug_key, drop = FALSE]), , drop = FALSE]
  reac <- reac[!duplicated(reac[, reaction_key, drop = FALSE]), , drop = FALSE]
  join_qa <- validate_report_join_budget(drug, reac)

  joined <- merge(
    drug[, drug_key, drop = FALSE],
    reac[, reaction_key, drop = FALSE],
    by = "primaryid",
    all = FALSE,
    sort = FALSE
  )
  metadata <- current_demo[, c("primaryid", "caseid", "quarter", demo_optional), drop = FALSE]
  metadata$primaryid <- as.character(metadata$primaryid)
  metadata$caseid <- as.character(metadata$caseid)
  pairs <- merge(joined, metadata, by = "primaryid", all = FALSE, sort = FALSE)

  key <- c("primaryid", "drug_text", "drug_text_source", "drug_role", "event_pt")
  pairs <- pairs[!duplicated(pairs[, key, drop = FALSE]), , drop = FALSE]
  pairs$event_category <- "UNCLASSIFIED_SOURCE_PT"
  pairs <- pairs[order(pairs$primaryid, pairs$drug_text, pairs$drug_role, pairs$event_pt), , drop = FALSE]
  rownames(pairs) <- NULL
  attr(pairs, "component_qa") <- c(
    eligible_drug_component_rows = eligible_drug_component_rows,
    unique_drug_components = nrow(drug),
    duplicate_drug_components_removed = eligible_drug_component_rows - nrow(drug),
    eligible_reaction_component_rows = eligible_reaction_component_rows,
    unique_reaction_components = nrow(reac),
    duplicate_reaction_components_removed = eligible_reaction_component_rows - nrow(reac),
    max_projected_pairs_per_report = join_qa$max_projected_pairs_per_report,
    reports_with_joinable_components = join_qa$reports_with_pairs
  )
  pairs
}

summarize_report_counts <- function(source_demo, current_demo, pairs) {
  assert_columns(source_demo, c("primaryid", "caseid", "caseversion"), "source DEMO")
  assert_columns(current_demo, c("primaryid", "caseid", "caseversion"), "current DEMO")
  assert_columns(pairs, "primaryid", "report-level pairs")

  source_ids <- canonical_positive_decimal(source_demo$primaryid, "source DEMO PRIMARYID")
  current_ids <- canonical_positive_decimal(current_demo$primaryid, "current DEMO PRIMARYID")
  eligible_ids <- unique(canonical_positive_decimal(pairs$primaryid, "report-level pair PRIMARYID"))
  if (anyDuplicated(current_ids)) {
    stop("Current DEMO must contain one retained row per report version", call. = FALSE)
  }
  if (any(!current_ids %in% source_ids)) {
    stop("Current DEMO contains a report that is absent from source DEMO", call. = FALSE)
  }
  if (any(!eligible_ids %in% current_ids)) {
    stop("Eligible report pairs contain a report that is not a retained current case version", call. = FALSE)
  }

  # These populations are deliberately non-interchangeable. Source DEMO rows
  # include superseded case versions; current case reports retain one version
  # per CASEID; eligible reports additionally require at least one retained
  # drug-role/event pair and form the disproportionality denominator.
  c(
    source_demo_rows = nrow(source_demo),
    current_case_reports = nrow(current_demo),
    eligible_reports = length(eligible_ids),
    drug_event_pairs = nrow(pairs)
  )
}

build_field_completeness <- function(eligible_demo, fields) {
  assert_columns(eligible_demo, fields, "eligible DEMO")
  denominator <- nrow(eligible_demo)
  if (denominator == 0L) {
    stop("Field completeness requires at least one eligible report", call. = FALSE)
  }
  lapply(fields, function(field) {
    values <- as.character(eligible_demo[[field]])
    missing <- sum(is.na(values) | !nzchar(trimws(values)))
    list(
      field = field,
      population = "eligible_reports",
      denominator_records = denominator,
      missing_records = missing,
      missing_percent = round(100 * missing / denominator, 6L)
    )
  })
}

build_aggregate <- function(pairs, dataset_id) {
  required <- c("primaryid", "drug_text", "drug_text_source", "drug_role", "event_pt")
  assert_columns(pairs, required, "report-level pairs")
  if (nrow(pairs) == 0L) {
    stop("No eligible report-level drug-event pairs remain after validation", call. = FALSE)
  }

  # Besides the four source roles, expose two pre-specified unions. Duplicating
  # rows before grouping is safe because every marginal below counts unique
  # PRIMARYID values; a report containing the same ingredient in PS and SS is
  # therefore counted once in the derived `suspect` group.
  suspect <- pairs[pairs$drug_role %in% c("primary_suspect", "secondary_suspect"), , drop = FALSE]
  suspect$drug_role <- "suspect"
  all_roles <- pairs
  all_roles$drug_role <- "all"
  analysis_pairs <- rbind(pairs, suspect, all_roles)
  analysis_pairs <- analysis_pairs[!duplicated(analysis_pairs[, required, drop = FALSE]), , drop = FALSE]

  count_reports <- function(x) length(unique(as.character(x)))
  a <- stats::aggregate(
    primaryid ~ drug_text + drug_text_source + drug_role + event_pt,
    analysis_pairs,
    count_reports
  )
  names(a)[names(a) == "primaryid"] <- "a"
  drug_total <- stats::aggregate(
    primaryid ~ drug_text + drug_text_source + drug_role,
    analysis_pairs,
    count_reports
  )
  names(drug_total)[names(drug_total) == "primaryid"] <- "drug_reports"
  event_total <- stats::aggregate(primaryid ~ event_pt, pairs, count_reports)
  names(event_total)[names(event_total) == "primaryid"] <- "event_reports"

  cells <- merge(a, drug_total, by = c("drug_text", "drug_text_source", "drug_role"), all.x = TRUE)
  cells <- merge(cells, event_total, by = "event_pt", all.x = TRUE)
  universe <- count_reports(pairs$primaryid)
  cells$universe_reports <- universe
  cells$b <- cells$drug_reports - cells$a
  cells$c <- cells$event_reports - cells$a
  cells$d <- universe - cells$a - cells$b - cells$c
  if (any(cells[, c("a", "b", "c", "d")] < 0)) {
    stop("Aggregate invariant failed: at least one 2x2 cell is negative", call. = FALSE)
  }
  cells$dataset_id <- dataset_id
  cells$comparator <- "all_other_eligible_reports"
  cells$event_scope <- "all_recorded_source_pts"
  cells$deduplication_policy <- "MAX_CASEVERSION_PER_CASEID_THEN_MAX_PRIMARYID"
  cells <- cells[, c(
    "dataset_id", "drug_text", "drug_text_source", "drug_role", "event_pt",
    "a", "b", "c", "d", "drug_reports", "event_reports", "universe_reports",
    "comparator", "event_scope", "deduplication_policy"
  )]
  cells <- cells[order(cells$drug_text, cells$drug_role, cells$event_pt), , drop = FALSE]
  rownames(cells) <- NULL
  cells
}

validate_tsv_formula_safety <- function(data) {
  character_columns <- names(data)[vapply(data, is.character, logical(1))]
  for (column in character_columns) {
    values <- enc2utf8(as.character(data[[column]]))
    dangerous <- !is.na(values) & grepl("^[=+@-]", values)
    if (any(dangerous)) {
      examples <- head(unique(values[dangerous]), 3L)
      stop(sprintf(
        "TSV field %s starts with a spreadsheet formula trigger: %s",
        column,
        paste(examples, collapse = ", ")
      ), call. = FALSE)
    }
  }
  invisible(TRUE)
}

classify_faers_filename <- function(path) {
  name <- toupper(basename(path))
  if (grepl("^DEMO.*\\.(TXT|CSV)$", name)) return("demo")
  if (grepl("^DRUG.*\\.(TXT|CSV)$", name)) return("drug")
  if (grepl("^REAC.*\\.(TXT|CSV)$", name)) return("reac")
  NA_character_
}

extract_faers_table_quarter <- function(path) {
  name <- toupper(basename(path))
  match <- regexec(
    "^(DEMO|DRUG|REAC)(20[0-9]{2}|[0-9]{2})Q([1-4])([^0-9].*)?\\.(TXT|CSV)$",
    name,
    perl = TRUE
  )
  fields <- regmatches(name, match)[[1L]]
  if (length(fields) == 0L) {
    stop(sprintf("FAERS table filename does not encode an unambiguous year and quarter: %s", basename(path)), call. = FALSE)
  }
  year <- fields[[3L]]
  if (nchar(year) == 2L) year <- paste0("20", year)
  paste0(year, "Q", fields[[4L]])
}

validate_faers_table_quarters <- function(paths, expected_quarter) {
  observed <- vapply(paths, extract_faers_table_quarter, character(1))
  if (any(observed != expected_quarter)) {
    details <- sprintf("%s=%s", basename(paths), observed)
    stop(sprintf(
      "%s source register does not match table filename quarter(s): %s",
      expected_quarter,
      paste(details, collapse = ", ")
    ), call. = FALSE)
  }
  invisible(TRUE)
}

validate_archive_entries <- function(entries) {
  normalized <- gsub("\\\\", "/", entries)
  unsafe <- grepl("^/|^[A-Za-z]:|(^|/)\\.\\.(/|$)", normalized)
  if (any(unsafe)) {
    stop(sprintf("Archive contains unsafe path(s): %s", paste(entries[unsafe], collapse = ", ")), call. = FALSE)
  }
  invisible(TRUE)
}

archive_extraction_limits <- function() {
  list(
    max_members = 128L,
    max_uncompressed_bytes = 16 * 1024^3,
    max_expansion_ratio = 100
  )
}

validate_archive_budget <- function(listing, archive_bytes, limits = archive_extraction_limits()) {
  assert_columns(listing, c("Name", "Length"), "ZIP central directory")
  archive_bytes <- as.numeric(archive_bytes)
  lengths <- suppressWarnings(as.numeric(listing$Length))
  if (length(archive_bytes) != 1L || !is.finite(archive_bytes) || archive_bytes <= 0 ||
      any(!is.finite(lengths)) || any(lengths < 0)) {
    stop("ZIP size metadata is invalid", call. = FALSE)
  }
  if (nrow(listing) > limits$max_members) {
    stop(sprintf("ZIP exceeds member budget: %d > %d", nrow(listing), limits$max_members), call. = FALSE)
  }
  total_uncompressed <- sum(lengths)
  if (!is.finite(total_uncompressed) || total_uncompressed > limits$max_uncompressed_bytes) {
    stop(sprintf(
      "ZIP exceeds uncompressed-size budget: %.0f > %.0f bytes",
      total_uncompressed,
      limits$max_uncompressed_bytes
    ), call. = FALSE)
  }
  expansion_ratio <- total_uncompressed / archive_bytes
  if (!is.finite(expansion_ratio) || expansion_ratio > limits$max_expansion_ratio) {
    stop(sprintf(
      "ZIP exceeds expansion-ratio budget: %.2f > %.2f",
      expansion_ratio,
      limits$max_expansion_ratio
    ), call. = FALSE)
  }
  invisible(list(
    members = nrow(listing),
    uncompressed_bytes = total_uncompressed,
    expansion_ratio = expansion_ratio
  ))
}

materialize_quarter_tables <- function(register_rows, working_directory) {
  quarter <- unique(register_rows$quarter)
  if (length(quarter) != 1L) stop("Internal error: source rows span more than one quarter", call. = FALSE)
  paths <- register_rows$.resolved_path
  zip_rows <- grepl("\\.zip$", paths, ignore.case = TRUE)

  if (any(zip_rows)) {
    if (length(paths) != 1L || !zip_rows[[1]]) {
      stop(sprintf("%s must use either one ZIP or three ASCII files, not a mixture", quarter), call. = FALSE)
    }
    listing_info <- utils::unzip(paths[[1]], list = TRUE)
    validate_archive_entries(listing_info$Name)
    validate_archive_budget(listing_info, file.info(paths[[1]])$size)
    listing <- listing_info$Name
    kinds <- vapply(listing, classify_faers_filename, character(1))
    selected <- listing[!is.na(kinds)]
    kinds <- kinds[!is.na(kinds)]
    for (kind in c("demo", "drug", "reac")) {
      if (sum(kinds == kind) != 1L) {
        stop(sprintf("%s ZIP must contain exactly one %s table", quarter, toupper(kind)), call. = FALSE)
      }
    }
    target <- file.path(working_directory, quarter)
    dir.create(target, recursive = TRUE, showWarnings = FALSE)
    utils::unzip(paths[[1]], files = selected, exdir = target, overwrite = FALSE)
    paths <- file.path(target, selected)
  }

  kinds <- vapply(paths, classify_faers_filename, character(1))
  for (kind in c("demo", "drug", "reac")) {
    if (sum(kinds == kind) != 1L) {
      stop(sprintf("%s requires exactly one %s file", quarter, toupper(kind)), call. = FALSE)
    }
  }
  validate_faers_table_quarters(paths, quarter)
  stats::setNames(as.list(paths[match(c("demo", "drug", "reac"), kinds)]), c("demo", "drug", "reac"))
}

expected_quarter_coverage <- function(quarter) {
  if (length(quarter) != 1L || !grepl("^20[0-9]{2}Q[1-4]$", quarter)) {
    stop("Invalid quarter; expected YYYYQ1..YYYYQ4", call. = FALSE)
  }
  year <- as.integer(substr(quarter, 1L, 4L))
  quarter_number <- as.integer(substr(quarter, 6L, 6L))
  start_month <- 1L + (quarter_number - 1L) * 3L
  start <- as.Date(sprintf("%04d-%02d-01", year, start_month))
  next_year <- if (quarter_number == 4L) year + 1L else year
  next_month <- if (quarter_number == 4L) 1L else start_month + 3L
  end <- as.Date(sprintf("%04d-%02d-01", next_year, next_month)) - 1
  c(start = format(start, "%Y-%m-%d"), end = format(end, "%Y-%m-%d"))
}

validate_source_register_metadata <- function(register) {
  required <- c("quarter", "file", "source_url", "retrieved_at", "coverage_start", "coverage_end", "sha256")
  assert_columns(register, required, "source register")
  if (nrow(register) == 0L) stop("Source register has no files", call. = FALSE)
  if (any(!grepl("^20[0-9]{2}Q[1-4]$", register$quarter))) stop("Invalid quarter; expected YYYYQ1..YYYYQ4", call. = FALSE)
  official_fda_url <- grepl(
    "^https://([A-Za-z0-9-]+\\.)*fda\\.gov(?::443)?(?:/|\\?|#|$)",
    register$source_url,
    perl = TRUE,
    ignore.case = TRUE
  )
  if (any(!official_fda_url)) {
    stop("Every formal FAERS source_url must be official HTTPS on fda.gov or a subdomain", call. = FALSE)
  }
  if (any(!grepl("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$", register$retrieved_at))) {
    stop("Every retrieved_at must be an explicit UTC timestamp (YYYY-MM-DDTHH:MM:SSZ)", call. = FALSE)
  }
  if (any(!grepl("^[0-9]{4}-[0-9]{2}-[0-9]{2}$", register$coverage_start)) ||
      any(!grepl("^[0-9]{4}-[0-9]{2}-[0-9]{2}$", register$coverage_end))) {
    stop("Coverage dates must use YYYY-MM-DD", call. = FALSE)
  }
  starts <- as.Date(register$coverage_start)
  ends <- as.Date(register$coverage_end)
  if (any(is.na(starts)) || any(is.na(ends)) || any(starts > ends)) {
    stop("Every coverage range must contain valid dates with start <= end", call. = FALSE)
  }
  expected_coverage <- t(vapply(register$quarter, expected_quarter_coverage, character(2)))
  coverage_matches <- register$coverage_start == expected_coverage[, "start"] &
    register$coverage_end == expected_coverage[, "end"]
  if (any(!coverage_matches)) {
    bad <- unique(register$quarter[!coverage_matches])
    stop(sprintf(
      "Coverage must exactly match the declared calendar quarter: %s",
      paste(bad, collapse = ", ")
    ), call. = FALSE)
  }
  if (any(!grepl("^[A-Fa-f0-9]{64}$", register$sha256))) stop("Every source must have a 64-character SHA-256", call. = FALSE)
  invisible(TRUE)
}

parse_named_arguments <- function(args) {
  if (length(args) %% 2L != 0L) stop("Arguments must be supplied as --name value pairs", call. = FALSE)
  keys <- args[seq(1L, length(args), by = 2L)]
  values <- args[seq(2L, length(args), by = 2L)]
  if (any(!startsWith(keys, "--"))) stop("Argument names must start with --", call. = FALSE)
  stats::setNames(as.list(values), substring(keys, 3L))
}
