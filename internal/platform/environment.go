package platform

import (
	"os"
	"runtime"
)

type EnvironmentVariable struct {
	Name         string `json:"name"`
	Value        string `json:"value"`
	Source       string `json:"source"`
	UserValue    string `json:"userValue"`
	SystemValue  string `json:"systemValue"`
	ProcessValue string `json:"processValue"`
	Error        string `json:"error"`
}

var ReadSystemVariable = func(string) (string, bool, error) { return "", false, nil }
var EnvironmentNames = func() []string { return nil }
var InspectVariable = inspectEnvironment
var ExpandVariableValue = func(value string) string { return value }

func inspectEnvironment(name string) EnvironmentVariable {
	result := EnvironmentVariable{Name: name, ProcessValue: os.Getenv(name)}
	if runtime.GOOS != "windows" {
		value, exists, err := readUnixStoredVariable(name)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		if exists {
			result.UserValue = value
			result.Value = value
			result.Source = "user"
		} else {
			result.Value = result.ProcessValue
			if result.Value != "" {
				result.Source = "session"
			}
		}
		return result
	}
	user, userExists, err := ReadVariable(name)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	machine, machineExists, err := ReadSystemVariable(name)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.UserValue = user
	result.SystemValue = machine
	switch {
	case userExists:
		result.Value = ExpandVariableValue(user)
		result.Source = "user"
	case machineExists:
		result.Value = ExpandVariableValue(machine)
		result.Source = "system"
	default:
		result.Value = result.ProcessValue
		if result.Value != "" {
			result.Source = "session"
		}
	}
	return result
}

func JavaEnvironment() []EnvironmentVariable {
	result := []EnvironmentVariable{}
	for _, name := range []string{"JAVA_HOME", "JDK_HOME", "GRAALVM_HOME", "JRE_HOME"} {
		result = append(result, InspectVariable(name))
	}
	return result
}
