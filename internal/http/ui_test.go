package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takayoshiotake/shiroyagi/internal/auth"
	mailimap "github.com/takayoshiotake/shiroyagi/internal/imap"
	"github.com/takayoshiotake/shiroyagi/internal/mailaccount"
)

func TestStaticAssetsAreServedWithoutAuthentication(t *testing.T) {
	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/static/css/app.css", contentType: "text/css; charset=utf-8", contains: "--space-14: 56px"},
		{path: "/static/js/htmx.min.js", contentType: "text/javascript; charset=utf-8", contains: "var htmx="},
	}

	handler := (&Server{}).Routes()
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if got := recorder.Header().Get("Content-Type"); got != test.contentType {
				t.Fatalf("Content-Type = %q, want %q", got, test.contentType)
			}
			if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
			}
			if !strings.Contains(recorder.Body.String(), test.contains) {
				t.Fatalf("body does not contain %q", test.contains)
			}
		})
	}
}

func TestStaticAssetAllowlistRejectsUnknownFiles(t *testing.T) {
	recorder := httptest.NewRecorder()
	(&Server{}).Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/static/unknown.txt", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestMailPageTemplateRendersProgressiveAndHTMXNavigation(t *testing.T) {
	view := mailPageView{
		Title:      "A <subject>",
		Mailbox:    "INBOX",
		MailboxURL: "/mail-accounts/account-1/mailboxes/INBOX",
		CurrentAccount: mailAccountView{
			Name:       "Account",
			Email:      "user@example.test",
			InboxURL:   "/mail-accounts/account-1/mailboxes/INBOX",
			ComposeURL: "/mail-accounts/account-1/send",
			Current:    true,
		},
		Accounts:    []mailAccountView{{Name: "Account", Email: "user@example.test", InboxURL: "/mail-accounts/account-1/mailboxes/INBOX", Current: true}},
		User:        mailUserView{Initial: "U"},
		Messages:    []mailMessageListItemView{{UID: 42, URL: "/mail-accounts/account-1/mailboxes/INBOX/messages/42", From: "Sender", Subject: "A <subject>", DateTime: "2026-08-02T10:00:00+09:00", DisplayDate: "10:00", Selected: true}},
		SelectedUID: 42,
		Selected: &mailMessageDetailView{
			UID: 42, Subject: "A <subject>", SenderName: "Sender", SenderEmail: "sender@example.test", SenderInitial: "S", To: "user@example.test", Body: "<script>alert(1)</script>",
			Attachments: []mailAttachmentView{{Filename: "../<original>.txt", ContentType: "text/plain", Size: "4 bytes", URL: "/messages/42/attachments/2"}},
		},
	}
	recorder := httptest.NewRecorder()
	if err := renderUI(recorder, http.StatusOK, "mail-page", view); err != nil {
		t.Fatal(err)
	}

	body := recorder.Body.String()
	for _, want := range []string{
		`href="/mail-accounts/account-1/mailboxes/INBOX/messages/42"`,
		`hx-get="/mail-accounts/account-1/mailboxes/INBOX/messages/42"`,
		`hx-target="#message-detail"`,
		`hx-history="false"`,
		`id="mail-loading"`,
		`aria-current="true"`,
		`A &lt;subject&gt;`,
		`&lt;script&gt;alert(1)&lt;/script&gt;`,
		`../&lt;original&gt;.txt`,
		`href="/messages/42/attachments/2"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered page does not contain %q", want)
		}
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("message body was rendered as executable HTML")
	}
}

func TestMessageDetailResponseUpdatesSelectionOutOfBand(t *testing.T) {
	view := mailPageView{
		Title:       "Selected",
		MailboxURL:  "/mail-accounts/account-1/mailboxes/INBOX",
		SelectedUID: 8,
		Selected:    &mailMessageDetailView{UID: 8, Subject: "Selected", SenderName: "Sender", SenderInitial: "S"},
		OOBRows: []mailMessageListItemView{
			{UID: 7, URL: "/messages/7", Subject: "Previous"},
			{UID: 8, URL: "/messages/8", Subject: "Selected", Selected: true},
		},
	}
	recorder := httptest.NewRecorder()
	if err := renderUI(recorder, http.StatusOK, "message-detail-response", view); err != nil {
		t.Fatal(err)
	}

	body := recorder.Body.String()
	if got := strings.Count(body, `hx-swap-oob="outerHTML"`); got != 3 {
		t.Fatalf("out-of-band swaps = %d, want 3", got)
	}
	if !strings.Contains(body, `id="selected-message" name="previous_uid" value="8"`) {
		t.Error("response does not preserve selected UID")
	}
}

func TestMailEmptyAndErrorStates(t *testing.T) {
	emptyRecorder := httptest.NewRecorder()
	if err := renderUI(emptyRecorder, http.StatusOK, "mail-page", mailPageView{
		Title: "INBOX", Mailbox: "INBOX", CurrentAccount: mailAccountView{ComposeURL: "/send"},
	}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"No messages", "No message selected"} {
		if !strings.Contains(emptyRecorder.Body.String(), want) {
			t.Errorf("empty page does not contain %q", want)
		}
	}

	errorRecorder := httptest.NewRecorder()
	if err := renderUI(errorRecorder, http.StatusBadGateway, "message-panel-error", mailPanelErrorView{Error: "Temporary <failure>"}); err != nil {
		t.Fatal(err)
	}
	if errorRecorder.Code != http.StatusBadGateway {
		t.Fatalf("error status = %d, want %d", errorRecorder.Code, http.StatusBadGateway)
	}
	errorBody := errorRecorder.Body.String()
	if !strings.Contains(errorBody, `role="alert"`) || !strings.Contains(errorBody, "Temporary &lt;failure&gt;") {
		t.Fatalf("unexpected error fragment: %s", errorBody)
	}
}

func TestMailViewFormattingAndHTMXDetection(t *testing.T) {
	message := newMailMessageListItemView("account 1", "INBOX", mailimap.MessageSummary{
		UID: 9, Subject: "", From: "Sender", Date: time.Date(2025, 1, 2, 3, 4, 0, 0, time.UTC), Answered: true,
	}, true)
	if message.Subject != "(no subject)" || message.Status != "Replied" || !message.Selected {
		t.Fatalf("unexpected message view: %#v", message)
	}
	if !strings.Contains(message.URL, "/messages/9") {
		t.Fatalf("message URL = %q", message.URL)
	}
	if !isHTMXRequest("true", "") {
		t.Error("regular HTMX request was not detected")
	}
	if isHTMXRequest("true", "true") {
		t.Error("history restore request must receive a full page")
	}
}

func TestMailEntryURL(t *testing.T) {
	if got := mailEntryURL(nil); got != "/settings/mail-accounts" {
		t.Fatalf("mailEntryURL(nil) = %q", got)
	}
	if got := mailEntryURL([]mailaccount.Summary{{ID: "account-1", EmailAddress: "user@example.test"}}); got != "/settings/mail-accounts" {
		t.Fatalf("mailEntryURL(unconfigured account) = %q", got)
	}
	if got := mailEntryURL([]mailaccount.Summary{
		{ID: "account-1", EmailAddress: "a@example.test"},
		{ID: "account-2", EmailAddress: "b@example.test", HasIMAP: true},
	}); got != "/mail-accounts/account-2/mailboxes/INBOX" {
		t.Fatalf("mailEntryURL(account) = %q", got)
	}
}

func TestIndexRedirectsToMailEntry(t *testing.T) {
	recorder := httptest.NewRecorder()
	(&Server{}).handleIndex(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusFound || recorder.Header().Get("Location") != "/mail-accounts" {
		t.Fatalf("index response = %d %q", recorder.Code, recorder.Header().Get("Location"))
	}
}

func TestAuthenticatedPagesAreNotStored(t *testing.T) {
	sessions := auth.NewSessionStore()
	sessions.Put("session-1", auth.UserSession{Subject: "user-1"})
	server := &Server{sessions: sessions}
	handler := server.requireSession(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/mail-accounts", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: "session-1"})

	handler(recorder, request)

	if got := recorder.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestAccountSettingsUseSharedVisualPatternsWithoutMailNavigation(t *testing.T) {
	view := accountSettingsPageView{
		Title: "Mail accounts",
		CurrentAccount: mailAccountView{
			Name: "user", Email: "user@example.test", InboxURL: "/mail-accounts/account-1/mailboxes/INBOX", ComposeURL: "/mail-accounts/account-1/send", Current: true,
		},
		User:     mailUserView{Initial: "U"},
		Settings: []accountSettingsItemView{{Email: "user@example.test", InboxURL: "/inbox", IMAPURL: "/imap", SMTPURL: "/smtp", DeleteURL: "/delete"}},
	}
	recorder := httptest.NewRecorder()
	if err := renderUI(recorder, http.StatusOK, "account-settings-page", view); err != nil {
		t.Fatal(err)
	}

	body := recorder.Body.String()
	for _, want := range []string{
		`class="minimal-topbar"`, `>Shiroyagi</span>`, `class="user-menu"`, `href="/auth/logout"`, `>Sign out</a>`,
		`class="settings-standalone"`, `class="settings-page-head"`,
		`href="/mail-accounts/account-1/mailboxes/INBOX"`, `href="/delete"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("settings page does not contain %q", want)
		}
	}
	if strings.Contains(body, `class="topbar"`) || strings.Contains(body, `class="mailboxes`) || strings.Contains(body, `class="account-switch"`) {
		t.Error("settings page contains mail workspace navigation")
	}
}

func TestRemovalConfirmationRequiresExplicitPost(t *testing.T) {
	view := accountSettingsPageView{
		Title: "Confirm removal", User: mailUserView{Initial: "U"},
		Confirmation: &removalConfirmationView{
			Heading: "Delete mail account?", Description: "user@example.test will be deleted.", Action: "/delete", CancelURL: "/settings", ConfirmLabel: "Delete account",
		},
	}
	recorder := httptest.NewRecorder()
	if err := renderUI(recorder, http.StatusOK, "account-removal-page", view); err != nil {
		t.Fatal(err)
	}
	body := recorder.Body.String()
	for _, want := range []string{`method="post" action="/delete"`, `href="/settings"`, "Delete mail account?", "Delete account"} {
		if !strings.Contains(body, want) {
			t.Errorf("confirmation page does not contain %q", want)
		}
	}
}

func TestSettingsValidationPreservesSafeValuesAndNotPasswords(t *testing.T) {
	view := accountSettingsPageView{
		Title: "IMAP settings", User: mailUserView{Initial: "U"},
		Protocol: &protocolSettingsView{
			Name: "IMAP", Email: "user@example.test", FormAction: "/imap/save", Host: "imap.example.test", Port: "invalid", Username: "user@example.test",
			Errors: map[string]string{"port": "Enter a valid port.", "password": "Enter the password."}, FormError: "Check the highlighted fields.",
		},
	}
	recorder := httptest.NewRecorder()
	if err := renderUI(recorder, http.StatusUnprocessableEntity, "account-protocol-page", view); err != nil {
		t.Fatal(err)
	}
	body := recorder.Body.String()
	for _, want := range []string{`value="imap.example.test"`, `value="invalid"`, `value="user@example.test"`, `aria-invalid="true"`, "Enter a valid port."} {
		if !strings.Contains(body, want) {
			t.Errorf("validation page does not contain %q", want)
		}
	}
	if strings.Contains(body, `name="imap_password" value=`) {
		t.Error("validation page redisplays a password")
	}
}

func TestNewAccountValidationPreservesEmail(t *testing.T) {
	view := accountSettingsPageView{
		Title: "Add mail account", User: mailUserView{Initial: "U"}, NewEmail: "invalid@example",
		Errors: map[string]string{"email_address": "Enter a valid email address."}, FormError: "Check the highlighted field.",
	}
	recorder := httptest.NewRecorder()
	if err := renderUI(recorder, http.StatusUnprocessableEntity, "account-new-page", view); err != nil {
		t.Fatal(err)
	}
	body := recorder.Body.String()
	for _, want := range []string{`value="invalid@example"`, `aria-describedby="email-address-error"`, "Enter a valid email address."} {
		if !strings.Contains(body, want) {
			t.Errorf("new account validation page does not contain %q", want)
		}
	}
}

func TestComposeValidationPreservesSafeValues(t *testing.T) {
	view := composePageView{
		Title: "Compose message", Heading: "Compose message", From: "from@example.test", FormAction: "/send", Mode: composeModeNew,
		To: "to@example.test", Subject: "", Body: "Draft body", SubmitLabel: "Send",
		Errors: map[string]string{"subject": "Enter a subject."}, FormError: "Check the highlighted fields.",
	}
	recorder := httptest.NewRecorder()
	if err := renderUI(recorder, http.StatusUnprocessableEntity, "compose-page", view); err != nil {
		t.Fatal(err)
	}
	body := recorder.Body.String()
	for _, want := range []string{`value="to@example.test"`, "Draft body", `aria-describedby="compose-subject-error"`, "Enter a subject."} {
		if !strings.Contains(body, want) {
			t.Errorf("compose validation page does not contain %q", want)
		}
	}
}

func TestAccountSettingsEmptyStateHasAddAccountNavigation(t *testing.T) {
	recorder := httptest.NewRecorder()
	if err := renderUI(recorder, http.StatusOK, "account-settings-page", accountSettingsPageView{Title: "Mail accounts"}); err != nil {
		t.Fatal(err)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "No mail accounts") || !strings.Contains(body, `href="/mail-accounts/new"`) {
		t.Fatalf("unexpected empty settings page: %s", body)
	}
	if strings.Contains(body, "Back to Inbox") {
		t.Error("empty settings page links back to an unavailable Inbox")
	}
}

func TestProtocolSettingsDoNotRedisplayPassword(t *testing.T) {
	view := accountSettingsPageView{
		Title:    "IMAP settings",
		Protocol: &protocolSettingsView{Name: "IMAP", Email: "user@example.test", FormAction: "/imap/save", HasConfig: true, Host: "imap.example.test", Port: "993", Security: "imaps", Username: "user"},
	}
	recorder := httptest.NewRecorder()
	if err := renderUI(recorder, http.StatusOK, "account-protocol-page", view); err != nil {
		t.Fatal(err)
	}

	body := recorder.Body.String()
	for _, want := range []string{`name="imap_host"`, `name="imap_port"`, `name="imap_security"`, `name="imap_username"`, `name="imap_password"`, "Leave blank to keep the current password."} {
		if !strings.Contains(body, want) {
			t.Errorf("IMAP settings do not contain %q", want)
		}
	}
	if strings.Contains(body, `name="imap_password" value=`) {
		t.Error("password field contains a redisplayed value")
	}
}

func TestSMTPSettingsDefaultToPlainSecurity(t *testing.T) {
	view := accountSettingsPageView{
		Title:    "SMTP settings",
		Protocol: &protocolSettingsView{Name: "SMTP", Email: "user@example.test", FormAction: "/smtp/save"},
	}
	recorder := httptest.NewRecorder()
	if err := renderUI(recorder, http.StatusOK, "account-protocol-page", view); err != nil {
		t.Fatal(err)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `name="smtp_security"`) || !strings.Contains(body, `<option value="plain" selected>Plain</option>`) {
		t.Fatalf("SMTP settings do not default to plain security: %s", body)
	}
	if !strings.Contains(body, `name="smtp_password" autocomplete="new-password" required`) {
		t.Fatal("new SMTP configuration does not require a password")
	}
}

func TestComposePageUsesFocusedSharedForm(t *testing.T) {
	view := composePageView{
		Title: "Compose message", User: mailUserView{Initial: "U"}, Heading: "Compose message",
		BackURL: "/inbox", From: "user@example.test", FormAction: "/send", Mode: composeModeNew,
		To: "to@example.test", Cc: "cc@example.test", Subject: "A <subject>", Body: "<script>alert(1)</script>", SubmitLabel: "Send", AttachmentHelp: "Up to 10 files.",
	}
	recorder := httptest.NewRecorder()
	if err := renderUI(recorder, http.StatusOK, "compose-page", view); err != nil {
		t.Fatal(err)
	}

	body := recorder.Body.String()
	for _, want := range []string{
		`class="minimal-topbar"`, `href="/auth/logout"`, `name="mode" value="new"`,
		`name="cc" value="cc@example.test"`, `A &lt;subject&gt;`, `&lt;script&gt;alert(1)&lt;/script&gt;`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("compose page does not contain %q", want)
		}
	}
	if strings.Contains(body, `class="mailboxes`) || strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("compose page contains mail navigation or unescaped message HTML")
	}
}

func TestReplyComposePagePreservesOriginalMetadata(t *testing.T) {
	original := composeOriginalView{From: "sender@example.test", MessageID: "<message@example.test>"}
	view := composePageView{
		Title: "Reply message", Heading: "Reply message", BackURL: "/message", FormAction: "/send",
		Mode: composeModeReply, Mailbox: "INBOX", UID: 42, SubmitLabel: "Send reply", Original: &original,
	}
	recorder := httptest.NewRecorder()
	if err := renderUI(recorder, http.StatusOK, "compose-page", view); err != nil {
		t.Fatal(err)
	}

	body := recorder.Body.String()
	for _, want := range []string{`name="mode" value="reply"`, `name="mailbox" value="INBOX"`, `name="uid" value="42"`, "Original message details", "&lt;message@example.test&gt;"} {
		if !strings.Contains(body, want) {
			t.Errorf("reply page does not contain %q", want)
		}
	}
}

func TestComposeStatusAndProtocolErrorPages(t *testing.T) {
	statusRecorder := httptest.NewRecorder()
	if err := renderUI(statusRecorder, http.StatusOK, "compose-status-page", composeStatusPageView{
		Title: "Message sent", Heading: "Message sent", From: "from@example.test", To: "to@example.test", PrimaryURL: "/message", PrimaryLabel: "Back to message",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(statusRecorder.Body.String(), "Message sent") || !strings.Contains(statusRecorder.Body.String(), `href="/message"`) {
		t.Fatal("sent status page is missing confirmation or navigation")
	}

	errorRecorder := httptest.NewRecorder()
	if err := renderUI(errorRecorder, http.StatusBadGateway, "protocol-error-page", protocolErrorPageView{
		Title: "SMTP error", User: mailUserView{Initial: "U"}, Heading: "SMTP error", Message: "Temporary <failure>", EditURL: "/smtp/edit", EditLabel: "Edit SMTP settings",
	}); err != nil {
		t.Fatal(err)
	}
	if errorRecorder.Code != http.StatusBadGateway || !strings.Contains(errorRecorder.Body.String(), "Temporary &lt;failure&gt;") || !strings.Contains(errorRecorder.Body.String(), `role="alert"`) {
		t.Fatal("protocol error page has incorrect status, escaping, or semantics")
	}
}

func TestSignInPageHasNoAuthenticatedUserMenu(t *testing.T) {
	recorder := httptest.NewRecorder()
	if err := renderUI(recorder, http.StatusOK, "sign-in-page", signInPageView{Title: "Sign in"}); err != nil {
		t.Fatal(err)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `href="/auth/login"`) || strings.Contains(body, `href="/auth/logout"`) {
		t.Fatalf("unexpected sign-in page: %s", body)
	}
}
