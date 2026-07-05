package httpserver

import "testing"

func TestMailAccountAAD(t *testing.T) {
	gotEnvelopeAAD := string(envelopeAAD("user-1", "account-1"))
	wantEnvelopeAAD := "user-1:account-1"
	if gotEnvelopeAAD != wantEnvelopeAAD {
		t.Fatalf("envelopeAAD() = %q, want %q", gotEnvelopeAAD, wantEnvelopeAAD)
	}

	gotFieldAAD := string(fieldAAD("user-1", "account-1", aadFieldIMAPPassword))
	wantFieldAAD := "user-1:account-1:imap_password"
	if gotFieldAAD != wantFieldAAD {
		t.Fatalf("fieldAAD() = %q, want %q", gotFieldAAD, wantFieldAAD)
	}
}

func TestParseMailAccountAction(t *testing.T) {
	id, action, ok := parseMailAccountAction("/mail-accounts/account-1/edit")
	if !ok {
		t.Fatal("parseMailAccountAction() ok = false, want true")
	}
	if id != "account-1" || action != "edit" {
		t.Fatalf("parseMailAccountAction() = %q, %q, want account-1, edit", id, action)
	}

	_, _, ok = parseMailAccountAction("/mail-accounts/new")
	if ok {
		t.Fatal("parseMailAccountAction(/mail-accounts/new) ok = true, want false")
	}
}

func TestSelected(t *testing.T) {
	if selected(true) != " selected" {
		t.Fatalf("selected(true) = %q, want selected attribute", selected(true))
	}
	if selected(false) != "" {
		t.Fatalf("selected(false) = %q, want empty string", selected(false))
	}
}
