package billing

import "testing"

func TestUserPrice(t *testing.T) {
	ten := 10.0
	p := Pricing{
		Settings: Settings{MarkupPercent: 5, FreeHitPrice: 100},
		Models: map[string]ModelPrice{
			"kenari:gpt-image-2":         {VendorPrice: 650},
			"kenari:claude-sonnet-5-5":   {VendorPrice: 100},
			"kenari:step-3-7-flash:free": {VendorPrice: 0},
			"kenari:nano-banana-2":       {VendorPrice: 333, MarkupPercent: &ten},
			"kenari:murah":               {VendorPrice: 0.5},
		},
	}
	cases := map[string]int64{
		"kenari:gpt-image-2":         683, // 650 * 1.05 = 682.5 → dibulatkan ke atas
		"kenari:claude-sonnet-5-5":   105, // tanpa galat float jadi 106
		"kenari:step-3-7-flash:free": 100, // gratis di vendor → tarif flat
		"pollinations:flux":          100, // belum diatur → tarif flat
		"kenari:nano-banana-2":       367, // markup khusus 10%: 366.3 → 367
		"kenari:murah":               1,
	}
	for model, want := range cases {
		if got := p.UserPrice(model); got != want {
			t.Errorf("UserPrice(%s) = %d, mau %d", model, got, want)
		}
	}
	if p.IsSet("pollinations:flux") || !p.IsSet("kenari:gpt-image-2") {
		t.Error("IsSet salah")
	}
}
