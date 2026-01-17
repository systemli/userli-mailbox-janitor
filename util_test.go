package main

import "testing"

func TestParseEmail(t *testing.T) {
	tests := []struct {
		name          string
		email         string
		wantLocalPart string
		wantDomain    string
		wantErr       bool
	}{
		{"valid email", "user@example.com", "user", "example.com", false},
		{"valid email with subdomain", "user@mail.example.com", "user", "mail.example.com", false},
		{"valid email with plus", "user+tag@example.com", "user+tag", "example.com", false},
		{"valid email with dots", "user.name@example.com", "user.name", "example.com", false},
		{"no at sign", "userexample.com", "", "", true},
		{"empty string", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localPart, domain, err := parseEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
				return
			}
			if localPart != tt.wantLocalPart {
				t.Errorf("parseEmail(%q) localPart = %v, want %v", tt.email, localPart, tt.wantLocalPart)
			}
			if domain != tt.wantDomain {
				t.Errorf("parseEmail(%q) domain = %v, want %v", tt.email, domain, tt.wantDomain)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		// Valid emails (allowlist)
		{"valid email", "user@example.com", false},
		{"valid email with subdomain", "user@mail.example.com", false},
		{"valid email with dots", "user.name@example.com", false},
		{"valid email with hyphen", "user-name@example.com", false},
		{"valid email with underscore", "user_name@example.com", false},
		{"valid email with numbers", "user123@example.com", false},
		{"valid email with start underscore", "_user123@example.com", false},
		{"valid email with start dash", "-user123@example.com", false},

		// Invalid emails (not matching allowlist)
		{"empty email", "", true},
		{"uppercase letters", "User@example.com", true},
		{"email with plus", "user+tag@example.com", true},
		{"email with dot start", ".user@example.com", true},
		{"space in email", "user @example.com", true},
		{"wildcard star", "*@example.com", true},
		{"wildcard question", "user?@example.com", true},
		{"wildcard in domain", "user@*.com", true},
		{"no at sign", "userexample.com", true},
		{"multiple at signs", "user@exam@ple.com", true},
		{"semicolon injection", "user@example.com;rm", true},
		{"pipe injection", "user@example.com|cat", true},
		{"ampersand injection", "user@example.com&whoami", true},
		{"backtick injection", "user@example.com`id`", true},
		{"dollar injection", "user@example.com$HOME", true},
		{"newline injection", "user@example.com\ninjected", true},
		{"quote injection", "user\"@example.com", true},
		{"single quote injection", "user'@example.com", true},
		{"double dot path traversal", "..@example.com", true},
		{"path traversal in local", "user..name@example.com", true},
		{"missing TLD", "user@example", true},
		{"single char TLD", "user@example.c", true},
		{"special chars", "user!@example.com", true},
		{"parentheses", "user()@example.com", true},
		{"brackets", "user[]@example.com", true},
		{"backslash", "user\\@example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestParseSieveScript(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "basic if true redirect",
			content: "if true {\n  redirect \"admin@example.com\";\n}",
			want:    true,
		},
		{
			name:    "if true redirect with spaces",
			content: "  if   true  {\n   redirect \"admin@example.com\";\n  }",
			want:    true,
		},
		{
			name:    "case insensitive IF TRUE REDIRECT",
			content: "IF TRUE {\n  REDIRECT \"admin@example.com\";\n}",
			want:    true,
		},
		{
			name:    "if true redirect with tabs",
			content: "\t\tif true {\n\t\t\tredirect \"admin@example.com\";\n\t\t}",
			want:    true,
		},
		{
			name:    "if true redirect with multiple lines",
			content: "# Comment\nif\ntrue\n{\nredirect\n\"admin@example.com\";\n}\n# Another comment",
			want:    true,
		},
		{
			name:    "if true redirect single line",
			content: "if true { redirect \"admin@example.com\"; }",
			want:    true,
		},
		{
			name:    "if true redirect with extra whitespace",
			content: "if    true    {    redirect    \"admin@example.com\";    }",
			want:    true,
		},
		{
			name:    "simple redirect without if true",
			content: "redirect \"admin@example.com\";",
			want:    false,
		},
		{
			name:    "redirect in different conditional",
			content: "if header :contains \"from\" \"spam@example.com\" {\n  redirect \"admin@example.com\";\n}",
			want:    false,
		},
		{
			name:    "if false redirect",
			content: "if false {\n  redirect \"admin@example.com\";\n}",
			want:    false,
		},
		{
			name:    "no redirect - fileinto",
			content: "if exists \"X-Spam-Flag\" {\n  fileinto \"Spam\";\n}",
			want:    false,
		},
		{
			name:    "no redirect - keep",
			content: "keep;",
			want:    false,
		},
		{
			name:    "no redirect - empty",
			content: "",
			want:    false,
		},
		{
			name:    "no redirect - comment only",
			content: "# This is a comment\n# Another comment",
			want:    false,
		},
		{
			name:    "no redirect - discard",
			content: "if header :contains \"subject\" \"spam\" {\n  discard;\n}",
			want:    false,
		},
		{
			name:    "redirect in comment should not match",
			content: "# This script would redirect if enabled\n# if true { redirect \"admin@example.com\"; }",
			want:    false,
		},
		{
			name:    "redirect as part of word should not match",
			content: "# This is about redirections and if truly amazing\nfileinto \"INBOX\";",
			want:    false,
		},
		{
			name:    "if true with other actions",
			content: "if true {\n  redirect \"admin@example.com\";\n  stop;\n}",
			want:    true,
		},
		{
			name:    "allof condition with redirect should not match",
			content: "if allof (header :contains \"subject\" \"test subject\") {\n  redirect \"admin@example.com\";\n}",
			want:    false,
		},
		{
			name:    "anyof condition with redirect should not match",
			content: "if anyof (header :contains \"from\" \"spam@test.com\",\n          header :contains \"subject\" \"urgent\") {\n  redirect \"admin@example.com\";\n}",
			want:    false,
		},
		{
			name:    "header contains condition with redirect should not match",
			content: "if header :contains \"subject\" \"test subject\" {\n  redirect \"admin@example.com\";\n}",
			want:    false,
		},
		{
			name:    "exists condition with redirect should not match",
			content: "if exists \"X-Spam-Flag\" {\n  redirect \"admin@example.com\";\n}",
			want:    false,
		},
		{
			name:    "size condition with redirect should not match",
			content: "if size :over 100K {\n  redirect \"admin@example.com\";\n}",
			want:    false,
		},
		{
			name:    "address condition with redirect should not match",
			content: "if address :is \"from\" \"test@example.com\" {\n  redirect \"admin@example.com\";\n}",
			want:    false,
		},
		{
			name:    "not condition with redirect should not match",
			content: "if not header :contains \"subject\" \"spam\" {\n  redirect \"admin@example.com\";\n}",
			want:    false,
		},
		{
			name:    "complex allof with multiple conditions should not match",
			content: "if allof (header :contains \"subject\" \"test\",\n          header :contains \"from\" \"sender@test.com\",\n          not exists \"X-Spam-Flag\") {\n  redirect \"admin@example.com\";\n}",
			want:    false,
		},
		{
			name:    "envelope condition with redirect should not match",
			content: "if envelope :contains \"to\" \"test@example.com\" {\n  redirect \"admin@example.com\";\n}",
			want:    false,
		},
		{
			name:    "body condition with redirect should not match",
			content: "if body :contains \"urgent message\" {\n  redirect \"admin@example.com\";\n}",
			want:    false,
		},
		{
			name:    "if true mixed with other conditions should still match",
			content: "if header :contains \"subject\" \"test\" {\n  fileinto \"Test\";\n}\nif true {\n  redirect \"admin@example.com\";\n}",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasGlobalRedirect([]byte(tt.content))
			if got != tt.want {
				t.Errorf("hasGlobalRedirect() = %v, want %v", got, tt.want)
			}
		})
	}
}
