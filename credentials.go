package screenshotscout

import "strings"

const (
	accessKeyVariable = "SCREENSHOTSCOUT_ACCESS_KEY"
	secretKeyVariable = "SCREENSHOTSCOUT_SECRET_KEY"
)

// resolvedCredentials holds the keys to validate, from the options or from the
// environment. A nil key was neither passed nor set.
type resolvedCredentials struct {
	accessKey *string
	secretKey *string
	// fromEnvironment reports whether the keys were read from the environment.
	fromEnvironment bool
}

// resolveCredentials picks the keys the client uses. Keys passed as options
// always win: if either one is passed, neither variable is read. Only when both
// are missing are both read from the environment.
func resolveCredentials(
	accessKey *string,
	secretKey *string,
	lookupEnv func(string) (string, bool),
) (resolvedCredentials, error) {
	if accessKey != nil || secretKey != nil {
		return resolvedCredentials{accessKey: accessKey, secretKey: secretKey}, nil
	}

	environmentAccessKey, err := readVariable(lookupEnv, accessKeyVariable)
	if err != nil {
		return resolvedCredentials{}, err
	}
	environmentSecretKey, err := readVariable(lookupEnv, secretKeyVariable)
	if err != nil {
		return resolvedCredentials{}, err
	}
	if environmentAccessKey == nil {
		return resolvedCredentials{}, &ConfigurationError{
			Message: "A non-blank Screenshot Scout access key is required. Pass one, or set " + accessKeyVariable + ".",
		}
	}
	return resolvedCredentials{
		accessKey:       environmentAccessKey,
		secretKey:       environmentSecretKey,
		fromEnvironment: true,
	}, nil
}

// readVariable returns nil for an unset or empty variable, and an error for one
// that holds only whitespace.
func readVariable(lookupEnv func(string) (string, bool), name string) (*string, error) {
	value, ok := lookupEnv(name)
	if !ok || value == "" {
		return nil, nil
	}
	if strings.TrimSpace(value) == "" {
		return nil, &ConfigurationError{Message: name + " is set but blank."}
	}
	return &value, nil
}
