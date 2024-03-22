package bitwarden

import (
	"os"
	"testing"
)

func TestGetAddressConfig(t *testing.T) {
	// Test case 1: cfgAddress is not empty
	cfgAddress := "http://example.com"
	expected := "http://example.com"
	result := getAddressConfig(cfgAddress)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}

	// Test case 2: cfgAddress is empty, but envAddr is not empty
	os.Setenv("BW_API_ADDR", "http://env.example.com")
	expected = "http://env.example.com"
	result = getAddressConfig("")
	os.Unsetenv("BW_API_ADDR")
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}

	// Test case 3: cfgAddress and envAddr are empty
	os.Setenv("BW_API_ADDR", "")
	expected = "http://localhost:8087"
	result = getAddressConfig("")
	os.Unsetenv("BW_API_ADDR")
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestExtractItemAndType(t *testing.T) {
	testCases := []struct {
		key              string
		expectedItemId   string
		expectedKeyType  string
		expectedErrorMsg string
	}{
		{
			key:              "item012",
			expectedItemId:   "item012",
			expectedKeyType:  "password",
			expectedErrorMsg: "",
		},
		{
			key:              "item123/password",
			expectedItemId:   "item123",
			expectedKeyType:  "password",
			expectedErrorMsg: "",
		},
		{
			key:              "item456/username",
			expectedItemId:   "item456",
			expectedKeyType:  "username",
			expectedErrorMsg: "",
		},
		{
			key:              "item789/invalid",
			expectedItemId:   "",
			expectedKeyType:  "",
			expectedErrorMsg: "bitwarden: get string: key \"item789/invalid\" unknown keytype \"invalid\"",
		},
		{
			key:              "",
			expectedItemId:   "",
			expectedKeyType:  "",
			expectedErrorMsg: "bitwarden: key cannot be empty",
		},
		{
			key:              "/password",
			expectedItemId:   "",
			expectedKeyType:  "",
			expectedErrorMsg: "bitwarden: key cannot be empty",
		},
	}

	for _, tc := range testCases {
		itemId, keyType, err := extractItemAndType(tc.key)
		if err != nil && err.Error() != tc.expectedErrorMsg {
			t.Errorf("Expected error message %q, but got %q", tc.expectedErrorMsg, err.Error())
		}
		if itemId != tc.expectedItemId {
			t.Errorf("Expected itemId %q, but got %q", tc.expectedItemId, itemId)
		}
		if keyType != tc.expectedKeyType {
			t.Errorf("Expected keyType %q, but got %q", tc.expectedKeyType, keyType)
		}
	}
}
