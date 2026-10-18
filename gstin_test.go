package main

import "testing"

func TestGSTIN(t *testing.T) {
	g := "27AAPFU0939F1Z"
	g += string(checkChar(g))
	if !validGSTIN(g) {
		t.Fatalf("%s should be valid", g)
	}
	for i := 2; i < 15; i++ {
		if i == 13 {
			continue
		}
		b := []byte(g)
		if b[i] == '0' {
			b[i] = '8'
		} else if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] = 'A' + (b[i]-'A'+1)%26
		} else {
			b[i] = '0'
		}
		if validGSTIN(string(b)) {
			t.Errorf("%s (char %d changed) should fail", b, i)
		}
	}
}

func TestCheck(t *testing.T) {
	sup := "27AAPFU0939F1Z"
	sup += string(checkChar(sup))
	buy := "27ABCDE1234F1Z"
	buy += string(checkChar(buy))
	clean := &Invoice{File: "a", SupplierGSTIN: sup, BuyerGSTIN: buy, InvoiceNumber: "1", TaxableValue: 1000, CGST: 90, SGST: 90, Total: 1180}
	wrongTax := &Invoice{File: "b", SupplierGSTIN: sup, BuyerGSTIN: buy, InvoiceNumber: "2", TaxableValue: 1000, IGST: 180, Total: 1180}
	dup := &Invoice{File: "c", SupplierGSTIN: sup, BuyerGSTIN: buy, InvoiceNumber: "1", TaxableValue: 1000, CGST: 90, SGST: 90, Total: 1180}
	check([]*Invoice{clean, wrongTax, dup}, buy)
	if len(clean.Issues) != 0 || len(wrongTax.Issues) != 1 || len(dup.Issues) != 1 {
		t.Fatalf("clean %v, wrongTax %v, dup %v", clean.Issues, wrongTax.Issues, dup.Issues)
	}
}

func TestRepair(t *testing.T) {
	g := "27ABZPI9054E1Z"
	g += string(checkChar(g))
	misread := g[:6] + "1" + g[7:]
	if got, ok := repairGSTIN(misread); !ok || got != g {
		t.Fatalf("repair(%s) = %s %v, want %s", misread, got, ok, g)
	}
	if _, ok := repairGSTIN("27AAKFL4410C1ZQ"); ok {
		t.Fatal("a wrong check digit must not be repaired")
	}
}
