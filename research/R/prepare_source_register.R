#!/usr/bin/env Rscript

`%||%` <- function(left, right) if (!is.null(left)) left else right
args <- commandArgs(trailingOnly = TRUE)
file_arg <- grep("^--file=", commandArgs(trailingOnly = FALSE), value = TRUE)
script_path <- if (length(file_arg) == 1L) sub("^--file=", "", file_arg) else "research/R/prepare_source_register.R"
root <- dirname(dirname(normalizePath(script_path, mustWork = FALSE)))
source(file.path(root, "R", "pipeline.R"), local = TRUE)

options(error = function() {
  traceback(2L)
  quit(status = 1L)
})

named <- parse_named_arguments(args)
required <- c("input", "output")
missing <- required[!required %in% names(named)]
if (length(missing) > 0L) stop(sprintf("Missing arguments: %s", paste(missing, collapse = ", ")), call. = FALSE)
if (!requireNamespace("digest", quietly = TRUE)) stop("Package 'digest' is required; run research/R/bootstrap.R", call. = FALSE)

input <- normalizePath(named$input, mustWork = TRUE)
output <- normalizePath(named$output, mustWork = FALSE)
if (file.exists(output)) stop("Refusing to overwrite an existing prepared source register", call. = FALSE)
register <- utils::read.csv(input, stringsAsFactors = FALSE, check.names = FALSE)
assert_columns(register, c("quarter", "file", "source_url", "retrieved_at", "coverage_start", "coverage_end"), "source register draft")
base <- dirname(input)
register$sha256 <- vapply(register$file, function(path) {
  resolved <- if (grepl("^[A-Za-z]:|^/", path)) path else file.path(base, path)
  resolved <- normalizePath(resolved, mustWork = TRUE)
  digest::digest(resolved, algo = "sha256", file = TRUE, serialize = FALSE)
}, character(1))
validate_source_register_metadata(register)
utils::write.csv(register, output, row.names = FALSE, na = "", fileEncoding = "UTF-8")
message(sprintf("Prepared %s with %d verified source row(s)", output, nrow(register)))
