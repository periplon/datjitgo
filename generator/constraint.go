package generator

// constraint.go currently just hosts shared helpers for uniqueness retry
// mechanics. The bulk of the retry loop lives inline in generateRow; this
// file keeps the package self-documenting — future phases may move more
// constraint-style logic here.
