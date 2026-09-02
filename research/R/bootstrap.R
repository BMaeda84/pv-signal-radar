#!/usr/bin/env Rscript

# Generate or restore the scientific R environment. The CRAN repository is a
# dated Posit Package Manager snapshot: package names alone are insufficiently
# reproducible when "latest" can resolve to different dependency graphs.

file_arg <- grep("^--file=", commandArgs(trailingOnly = FALSE), value = TRUE)
script_path <- if (length(file_arg) == 1L) sub("^--file=", "", file_arg) else "research/R/bootstrap.R"
research_root <- dirname(dirname(normalizePath(script_path, mustWork = FALSE)))
lockfile <- file.path(research_root, "renv.lock")

args <- commandArgs(trailingOnly = TRUE)
mode <- if (length(args) == 0L) "--restore" else args[[1L]]
if (length(args) > 1L || !mode %in% c("--restore", "--snapshot")) {
  stop("Usage: bootstrap.R [--restore|--snapshot]", call. = FALSE)
}

expected_r <- "4.6.1"
expected_bioconductor <- "3.23"
cran_repository <- "https://p3m.dev/cran/__linux__/noble/2026-09-02"
expected_packages <- c(
  arrow = "25.0.1",
  `data.table` = "1.18.6.1",
  digest = "0.6.39",
  faers = "1.8.0",
  jsonlite = "2.0.0",
  openEBGM = "0.9.1",
  pvda = "0.0.4",
  renv = "1.2.4"
)

if (as.character(getRversion()) != expected_r) {
  stop(sprintf("R version mismatch: expected %s; got %s", expected_r, getRversion()), call. = FALSE)
}
options(repos = c(CRAN = cran_repository))
# renv otherwise prefers the normalized source URL stored in the lock. The
# platform-qualified mirror serves the same dated snapshot as vetted binaries.
Sys.setenv(RENV_CONFIG_REPOS_OVERRIDE = cran_repository)

# renv must exist before it can read its own lock record. The dated repository
# and post-install version check make this bootstrap step deterministic too.
if (!requireNamespace("renv", quietly = TRUE) ||
    as.character(utils::packageVersion("renv")) != expected_packages[["renv"]]) {
  utils::install.packages("renv", repos = cran_repository)
}
if (as.character(utils::packageVersion("renv")) != expected_packages[["renv"]]) {
  stop("The CRAN snapshot did not provide the expected renv version", call. = FALSE)
}
renv::consent(provided = TRUE)

verify_environment <- function() {
  missing <- names(expected_packages)[
    !vapply(names(expected_packages), requireNamespace, logical(1), quietly = TRUE)
  ]
  if (length(missing) > 0L) {
    stop(sprintf("Missing packages after bootstrap: %s", paste(missing, collapse = ", ")), call. = FALSE)
  }

  actual <- vapply(
    names(expected_packages),
    function(package) as.character(utils::packageVersion(package)),
    character(1)
  )
  mismatched <- names(expected_packages)[actual != expected_packages]
  if (length(mismatched) > 0L) {
    details <- sprintf(
      "%s expected %s got %s",
      mismatched,
      expected_packages[mismatched],
      actual[mismatched]
    )
    stop(sprintf("Package version mismatch: %s", paste(details, collapse = "; ")), call. = FALSE)
  }

  bioc_version <- as.character(BiocManager::version())
  if (bioc_version != expected_bioconductor) {
    stop(
      sprintf("Bioconductor version mismatch: expected %s; got %s", expected_bioconductor, bioc_version),
      call. = FALSE
    )
  }
}

verify_lockfile <- function() {
  lock <- renv::lockfile_read(lockfile)
  if (lock$R$Version != expected_r || lock$Bioconductor$Version != expected_bioconductor) {
    stop("renv.lock does not match the pinned R/Bioconductor runtime", call. = FALSE)
  }

  locked <- vapply(lock$Packages, function(record) record$Version, character(1))
  missing <- names(locked)[
    !vapply(names(locked), requireNamespace, logical(1), quietly = TRUE)
  ]
  if (length(missing) > 0L) {
    stop(sprintf("Missing locked packages: %s", paste(missing, collapse = ", ")), call. = FALSE)
  }

  actual <- vapply(names(locked), function(package) {
    as.character(utils::packageVersion(package))
  }, character(1))
  # R normalizes separators in installed versions (for example 1.90.0-1 is
  # reported as 1.90.0.1), so compare package_version objects, not strings.
  same_version <- mapply(
    function(installed, recorded) utils::packageVersion(installed) == base::package_version(recorded),
    names(locked),
    locked,
    USE.NAMES = FALSE
  )
  mismatched <- names(locked)[!same_version]
  if (length(mismatched) > 0L) {
    details <- sprintf("%s locked %s got %s", mismatched, locked[mismatched], actual[mismatched])
    stop(sprintf("Lock restore mismatch: %s", paste(details, collapse = "; ")), call. = FALSE)
  }
}

if (mode == "--restore") {
  if (!file.exists(lockfile)) stop("research/renv.lock is required for restore", call. = FALSE)
  renv::restore(project = research_root, lockfile = lockfile, prompt = FALSE)
  verify_environment()
  verify_lockfile()
  message("Restored and verified the scientific R environment.")
  quit(save = "no", status = 0L)
}

if (file.exists(lockfile)) {
  stop("Refusing to overwrite an existing renv.lock; remove it only after reviewing the intended update", call. = FALSE)
}

renv::init(project = research_root, bare = TRUE, restart = FALSE)
renv::settings$bioconductor.version(expected_bioconductor, project = research_root)

# CRAN methods are version-qualified; faers is resolved from the fixed
# Bioconductor release, whose package version is verified before snapshotting.
cran_roots <- setdiff(names(expected_packages), c("faers", "renv"))
cran_specs <- sprintf("%s@%s", cran_roots, expected_packages[cran_roots])
renv::install(cran_specs, project = research_root)
renv::install("bioc::faers", project = research_root)
verify_environment()

# Supplying the roots explicitly records only their recursive runtime graph;
# type="all" would also capture unrelated packages preinstalled in the image.
renv::snapshot(
  project = research_root,
  lockfile = lockfile,
  packages = names(expected_packages),
  repos = c(CRAN = cran_repository),
  prompt = FALSE
)
verify_lockfile()
message("Generated and verified research/renv.lock.")
