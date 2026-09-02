# Independent, transparent 2x2 reference calculations used by fixture tests.
# Production signal labels must be tied to a pre-specified protocol; this file
# calculates measures and does not turn them into causal or regulatory claims.

calculate_reference_metrics <- function(a, b, c, d, zero_correction = 0.5) {
  cells <- as.numeric(c(a, b, c, d))
  if (length(cells) != 4L || any(!is.finite(cells)) || any(cells < 0) || any(cells != floor(cells))) {
    stop("a, b, c, and d must be finite non-negative integers", call. = FALSE)
  }
  corrected <- any(cells == 0)
  working <- if (corrected) cells + zero_correction else cells
  names(working) <- c("a", "b", "c", "d")
  prr <- (working["a"] / (working["a"] + working["b"])) /
    (working["c"] / (working["c"] + working["d"]))
  ror <- (working["a"] * working["d"]) / (working["b"] * working["c"])
  prr_se <- sqrt(1 / working["a"] - 1 / (working["a"] + working["b"]) +
    1 / working["c"] - 1 / (working["c"] + working["d"]))
  ror_se <- sqrt(sum(1 / working))
  # Use the same full standard-normal quantile as pvda and the Go runtime.
  # The former 1.96 shorthand produced deterministic but avoidable cross-engine
  # drift in the final interval digits.
  z_95 <- stats::qnorm(0.975)
  fisher <- stats::fisher.test(matrix(c(a, c, b, d), nrow = 2L), alternative = "two.sided")$p.value
  list(
    prr = unname(prr),
    prr_lower_95 = unname(exp(log(prr) - z_95 * prr_se)),
    prr_upper_95 = unname(exp(log(prr) + z_95 * prr_se)),
    ror = unname(ror),
    ror_lower_95 = unname(exp(log(ror) - z_95 * ror_se)),
    ror_upper_95 = unname(exp(log(ror) + z_95 * ror_se)),
    fisher_two_sided_p = unname(fisher),
    zero_correction_applied = corrected,
    zero_correction = if (corrected) zero_correction else 0
  )
}

adjust_fdr <- function(p_values) {
  if (any(!is.finite(p_values)) || any(p_values < 0 | p_values > 1)) {
    stop("p-values must be finite and between zero and one", call. = FALSE)
  }
  stats::p.adjust(p_values, method = "BH")
}
