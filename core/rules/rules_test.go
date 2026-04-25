package rules

import "testing"

func TestNormalizeIfThen(t *testing.T) {
	got := NormalizeExpr(" if User.age >= 18 then User.active ")
	want := "not (User.age >= 18) or (User.active)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeExprLeavesPlainExpressionsAlone(t *testing.T) {
	got := NormalizeExpr("User.age >= 18")
	want := "User.age >= 18"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
