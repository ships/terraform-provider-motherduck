package provider

import "testing"

func TestSqlBackedResource_TokenAttrAndClient(t *testing.T) {
	var b sqlBackedResource
	attr := b.tokenAttribute()
	if !attr.Required || !attr.Sensitive {
		t.Fatalf("token attribute must be required + sensitive")
	}
	if len(attr.Validators) == 0 {
		t.Fatalf("token attribute must reject the empty string")
	}
	if b.clientFor("tok") == nil {
		t.Fatalf("clientFor must build a SQL client")
	}
}

func TestSplitImportID(t *testing.T) {
	cases := []struct {
		id          string
		token, name string
		ok          bool
	}{
		{"tok,warehouse", "tok", "warehouse", true},
		{"ey.J.z,my_share", "ey.J.z", "my_share", true},
		{"tok,name_with,comma", "tok", "name_with,comma", true},
		{"warehouse", "", "", false},
		{",warehouse", "", "", false},
		{"tok,", "", "", false},
	}
	for _, c := range cases {
		token, name, ok := splitImportID(c.id)
		if ok != c.ok || token != c.token || name != c.name {
			t.Fatalf("splitImportID(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.id, token, name, ok, c.token, c.name, c.ok)
		}
	}
}
