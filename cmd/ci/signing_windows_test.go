package main

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWindowsSigningWithSelfSignedCertificate(t *testing.T) {
	if os.Getenv("TOOLS_SWITCHER_SIGNING_TEST") != "1" {
		t.Skip("enable TOOLS_SWITCHER_SIGNING_TEST for native signing and timestamp checks")
	}
	if _, err := windowsSignTool(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certificate := filepath.Join(dir, "test certificate.pfx")
	t.Setenv("SIGNING_TEST_CERTIFICATE", certificate)
	t.Setenv("WINDOWS_CERTIFICATE_PASSWORD", "temporary-test-password")
	powershell := func(script string) {
		t.Helper()
		command := exec.Command("pwsh", "-NoProfile", "-NonInteractive", "-Command", script)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("PowerShell: %v\n%s", err, output)
		}
	}
	powershell(`$ErrorActionPreference = 'Stop'
$rsa = [System.Security.Cryptography.RSA]::Create(2048)
$request = [System.Security.Cryptography.X509Certificates.CertificateRequest]::new('CN=tools-switcher signing test', $rsa, [System.Security.Cryptography.HashAlgorithmName]::SHA256, [System.Security.Cryptography.RSASignaturePadding]::Pkcs1)
$usages = [System.Security.Cryptography.OidCollection]::new()
[void]$usages.Add([System.Security.Cryptography.Oid]::new('1.3.6.1.5.5.7.3.3'))
$request.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509EnhancedKeyUsageExtension]::new($usages, $true))
$request.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509KeyUsageExtension]::new([System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]::DigitalSignature, $true))
$cert = $request.CreateSelfSigned([DateTimeOffset]::UtcNow.AddMinutes(-5), [DateTimeOffset]::UtcNow.AddDays(1))
[IO.File]::WriteAllBytes($env:SIGNING_TEST_CERTIFICATE, $cert.Export([System.Security.Cryptography.X509Certificates.X509ContentType]::Pfx, $env:WINDOWS_CERTIFICATE_PASSWORD))
$cert.Dispose()
$rsa.Dispose()`)
	data, err := os.ReadFile(certificate)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("WINDOWS_CERTIFICATE_BASE64", base64.StdEncoding.EncodeToString(data))
	source := filepath.Join(dir, "main.go")
	if err = os.WriteFile(source, []byte("package main\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "app.exe")
	command := exec.Command("go", "build", "-o", binary, source)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	installer := filepath.Join(dir, "setup.exe")
	copyFile(binary, installer)
	for _, file := range []string{binary, installer} {
		signWindows(file)
		t.Setenv("SIGNING_TEST_BINARY", file)
		powershell(`$ErrorActionPreference = 'Stop'
$signature = Get-AuthenticodeSignature -LiteralPath $env:SIGNING_TEST_BINARY
if ($signature.Status -notin @('Valid', 'NotTrusted', 'UnknownError')) { throw "Unexpected signature status: $($signature.Status) $($signature.StatusMessage)" }
if ($signature.SignerCertificate.Subject -ne 'CN=tools-switcher signing test') { throw 'Wrong signer' }
if ($null -eq $signature.TimeStamperCertificate) { throw 'Missing timestamp' }`)
	}
	file, err := os.OpenFile(installer, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteAt([]byte{1}, 64); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	powershell(`$signature = Get-AuthenticodeSignature -LiteralPath $env:SIGNING_TEST_BINARY
if ($signature.Status -ne 'HashMismatch') { throw "Modified binary accepted: $($signature.Status)" }`)
}
