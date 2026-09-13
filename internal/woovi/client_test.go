package woovi

import "testing"

func TestWithdrawFeeCents(t *testing.T) {
	cases := []struct {
		name    string
		balance int
		want    int
	}{
		{"zero balance", 0, 0},
		{"negative balance", -100, 0},
		{"one cent", 1, 100},
		{"just under R$500", 49999, 100},
		{"exactly R$500", 50000, 0},
		{"above R$500", 100000, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := WithdrawFeeCents(tc.balance); got != tc.want {
				t.Errorf("WithdrawFeeCents(%d) = %d, want %d", tc.balance, got, tc.want)
			}
		})
	}
}

func TestPixKeyType(t *testing.T) {
	cases := []struct {
		key  string
		want string
	}{
		{"driver@example.com", "EMAIL"},
		{"+5511999999999", "PHONE"},
		{"12345678901", "CPF"},
		{"12345678000199", "CNPJ"},
		{"550e8400-e29b-41d4-a716-446655440000", "RANDOM"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			if got := pixKeyType(tc.key); got != tc.want {
				t.Errorf("pixKeyType(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}
