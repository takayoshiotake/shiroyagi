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

	gotSMTPFieldAAD := string(fieldAAD("user-1", "account-1", aadFieldSMTPPassword))
	wantSMTPFieldAAD := "user-1:account-1:smtp_password"
	if gotSMTPFieldAAD != wantSMTPFieldAAD {
		t.Fatalf("fieldAAD() = %q, want %q", gotSMTPFieldAAD, wantSMTPFieldAAD)
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
