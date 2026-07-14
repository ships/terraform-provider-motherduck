package provider

import "testing"

func TestSqlBackedResource_TokenAttrAndClient(t *testing.T) {
	var b sqlBackedResource
	attr := b.tokenAttribute()
	if !attr.Required || !attr.Sensitive {
		t.Fatalf("token attribute must be required + sensitive")
	}
	if b.clientFor("tok") == nil {
		t.Fatalf("clientFor must build a SQL client")
	}
}
