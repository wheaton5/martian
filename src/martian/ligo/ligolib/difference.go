package ligolib

import "math"

/*
 * Implement the computation for percent difference. This is tricky when we're
 * not comparing a "right" answer to candidate answer since the usual
 * definition of percent difference is not symmetric.  Here we symmetricize the
 * result by comparing x-y to the average of x and y.
 *
 * Note that this isn't always well behavied either.  A better approach is to
 * use the Kolmogrov Smirnov statistic (see ~dstaff/ks-distance.go). However, I
 * chose not to use it because the interpretation is a little bit more subtle.
 */
func PercentDifference(x, y float64) float64 {
	if x == y {
		return 0
	}

	if x == -y {
		return math.Inf(1)
	}

	return math.Abs(2.0 * (x - y) / (x + y))
}
