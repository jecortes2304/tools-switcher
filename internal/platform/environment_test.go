package platform

import (
	"errors"
	"runtime"
	"testing"
)

func TestEnvironmentResolution(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows persistent scopes")
	}
	read, system := ReadVariable, ReadSystemVariable
	defer func() { ReadVariable = read; ReadSystemVariable = system }()
	t.Setenv("JAVA_HOME", "session-jdk")
	ReadVariable = func(string) (string, bool, error) { return "", false, nil }
	ReadSystemVariable = func(string) (string, bool, error) { return "system-jdk", true, nil }
	value := InspectVariable("JAVA_HOME")
	if value.Value != "system-jdk" || value.Source != "system" || value.ProcessValue != "session-jdk" {
		t.Fatalf("%+v", value)
	}
	ReadVariable = func(string) (string, bool, error) { return "user-jdk", true, nil }
	value = InspectVariable("JAVA_HOME")
	if value.Value != "user-jdk" || value.Source != "user" || value.SystemValue != "system-jdk" {
		t.Fatalf("%+v", value)
	}
	ReadSystemVariable = func(string) (string, bool, error) { return "", false, nil }
	ReadVariable = func(string) (string, bool, error) { return "", false, nil }
	value = InspectVariable("JAVA_HOME")
	if value.Value != "session-jdk" || value.Source != "session" {
		t.Fatalf("%+v", value)
	}
	ReadVariable = func(string) (string, bool, error) { return "", false, errors.New("access denied") }
	if InspectVariable("JAVA_HOME").Error == "" {
		t.Fatal("registry errors swallowed")
	}
}
