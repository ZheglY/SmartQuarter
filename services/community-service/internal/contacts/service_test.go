package contacts

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
	"testing"
)

func TestContactValidation(t *testing.T) {
	good := Input{Category: "EMERGENCY_DISPATCH", Title: "Диспетчерская", Phone: "+79991234567"}
	if e := Validate(&good); e != nil {
		t.Fatal(e)
	}
	tests := []func(*Input){func(v *Input) { v.Phone = "8(999)1234567" }, func(v *Input) { v.Phone = "+79991234567 ext123" }, func(v *Input) { v.Website = "javascript:alert(1)" }, func(v *Input) { v.Email = "Name <mail@example.test>" }, func(v *Input) { v.Description = "<script>alert(1)</script>" }, func(v *Input) { v.Category = "UNKNOWN" }, func(v *Input) { v.Title = strings.Repeat("я", 151); v.Description = v.Title }}
	for i, change := range tests {
		v := good
		change(&v)
		if status.Code(Validate(&v)) != codes.InvalidArgument {
			t.Fatalf("case %d accepted", i)
		}
	}
}
