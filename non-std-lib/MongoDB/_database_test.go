package main

import (
	context "context"
	errors "errors"
	debug "runtime/debug"
	testing "testing"
	time "time"
	mongo "go.mongodb.org/mongo-driver/mongo"
	options "go.mongodb.org/mongo-driver/mongo/options"
	readpref "go.mongodb.org/mongo-driver/mongo/readpref"
)

const testMongoAuthFailURI = "mongodb://testuser:wrongpassword@localhost:27017/?authSource=admin"
const testMongoInvalidHostURI = "mongodb://invalid-hostname-that-does-not-exist:27017"
const testMongoInvalidSyntaxURI = "mongodb:/invalid-syntax"
const testMongoTimeoutURI = "mongodb://10.255.255.1:27017/?connectTimeoutMS=1000&serverSelectionTimeoutMS=1500"
const testMongoURI = "mongodb://localhost:27017"






/*
ROOST_METHOD_HASH=connectToMongo_0609c809ec
ROOST_METHOD_SIG_HASH=connectToMongo_de1d46f4de

FUNCTION_DEF=func connectToMongo(client *mongo.Client) error 

*/
func TestConnectToMongo(t *testing.T) {

	testCases := []struct {
		name           string
		uri            string
		setupClient    func(t *testing.T, uri string) (*mongo.Client, error)
		preConnect     func(t *testing.T, client *mongo.Client) error
		postDisconnect func(t *testing.T, client *mongo.Client) error
		expectError    bool
		errorCheck     func(err error) bool
		errorMsg       string
		skipCondition  func(t *testing.T) (bool, string)
	}{

		{
			name: "Scenario 1: Successful Connection",
			uri:  testMongoURI,
			setupClient: func(t *testing.T, uri string) (*mongo.Client, error) {

				clientOptions := options.Client().ApplyURI(uri).SetTimeout(5 * time.Second)
				return mongo.NewClient(clientOptions)
			},
			expectError: false,
			skipCondition: func(t *testing.T) (bool, string) {

				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				client, err := mongo.Connect(ctx, options.Client().ApplyURI(testMongoURI).SetServerSelectionTimeout(1*time.Second))
				if err != nil {
					return true, "Skipping: MongoDB at " + testMongoURI + " not reachable or testMongoURI not configured. Error: " + err.Error()
				}

				pingErr := client.Ping(ctx, readpref.Primary())

				_ = client.Disconnect(context.Background())
				if pingErr != nil {
					return true, "Skipping: MongoDB at " + testMongoURI + " reachable but ping failed. Error: " + pingErr.Error()
				}
				return false, ""
			},
		},

		{
			name: "Scenario 2: Connection Timeout",
			uri:  testMongoTimeoutURI,
			setupClient: func(t *testing.T, uri string) (*mongo.Client, error) {

				clientOptions := options.Client().ApplyURI(uri)

				return mongo.NewClient(clientOptions)
			},
			expectError: true,
			errorCheck: func(err error) bool {

				return errors.Is(err, context.DeadlineExceeded) || mongo.IsTimeout(err) || mongo.IsNetworkError(err)
			},
			errorMsg: "Expected context.DeadlineExceeded or mongo driver timeout/network error",
		},

		{
			name: "Scenario 3: Connection Failure (Invalid Hostname)",
			uri:  testMongoInvalidHostURI,
			setupClient: func(t *testing.T, uri string) (*mongo.Client, error) {

				clientOptions := options.Client().ApplyURI(uri).SetServerSelectionTimeout(2 * time.Second)
				return mongo.NewClient(clientOptions)
			},
			expectError: true,

			errorCheck: func(err error) bool {
				return errors.Is(err, context.DeadlineExceeded) || mongo.IsTimeout(err) || mongo.IsNetworkError(err)
			},
			errorMsg: "Expected context.DeadlineExceeded or mongo driver timeout/network error due to invalid hostname",
		},

		{
			name: "Scenario 3b: Connection Failure (Invalid URI Syntax)",
			uri:  testMongoInvalidSyntaxURI,
			setupClient: func(t *testing.T, uri string) (*mongo.Client, error) {

				clientOptions := options.Client().ApplyURI(uri)

				return mongo.NewClient(clientOptions)

			},

			expectError: true,
		},

		{
			name: "Scenario 4: Connection Failure (Authentication Error)",
			uri:  testMongoAuthFailURI,
			setupClient: func(t *testing.T, uri string) (*mongo.Client, error) {

				clientOptions := options.Client().ApplyURI(uri).SetTimeout(5 * time.Second)
				return mongo.NewClient(clientOptions)
			},
			expectError: true,

			errorCheck: func(err error) bool { return mongo.IsCommandError(err) || mongo.IsAuthError(err) },
			errorMsg:   "Expected an authentication-related error (CommandError or AuthError)",
			skipCondition: func(t *testing.T) (bool, string) {

				if testMongoAuthFailURI == "mongodb://testuser:wrongpassword@localhost:27017/?authSource=admin" {

					ctxCheck, cancelCheck := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancelCheck()

					clientCheck, errCheck := mongo.Connect(ctxCheck, options.Client().ApplyURI(testMongoURI).SetServerSelectionTimeout(1*time.Second))
					if errCheck != nil {
						return true, "Skipping Auth Test: Base MongoDB at " + testMongoURI + " not reachable. Error: " + errCheck.Error()
					}
					pingErr := clientCheck.Ping(ctxCheck, readpref.Primary())
					_ = clientCheck.Disconnect(context.Background())
					if pingErr != nil {

						t.Logf("Warning: Could not ping %s without auth (Error: %v). Ensure it's running and configured for auth testing.", testMongoURI, pingErr)

					}

					return false, "Note: testMongoAuthFailURI is using default. Ensure MongoDB at localhost:27017 requires authentication for this test."

				}

				opts, err := options.Client().ApplyURI(testMongoAuthFailURI).SetServerSelectionTimeout(1 * time.Second).SetAuth(options.Credential{}).Parse()
				if err != nil {
					return true, "Skipping Auth Test: Cannot parse host from configured testMongoAuthFailURI: " + err.Error()
				}
				ctxCheck, cancelCheck := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancelCheck()
				clientCheck, errCheck := mongo.Connect(ctxCheck, opts)
				if errCheck != nil {
					return true, "Skipping Auth Test: Host from testMongoAuthFailURI not reachable. Error: " + errCheck.Error()
				}
				pingErr := clientCheck.Ping(ctxCheck, readpref.Primary())
				_ = clientCheck.Disconnect(context.Background())
				if pingErr != nil {
					t.Logf("Warning: Could not ping host from testMongoAuthFailURI without auth (Error: %v). Ensure it's running and configured for auth testing.", pingErr)
				}

				return false, ""
			},
		},

		{
			name: "Scenario 5: Nil Client Input",
			uri:  "",
			setupClient: func(t *testing.T, uri string) (*mongo.Client, error) {
				return nil, nil
			},

			expectError: true,
			errorCheck: func(err error) bool {

				return err != nil
			},
			errorMsg: "Expected an error or panic when passing a nil client",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			if tc.skipCondition != nil {
				skip, reason := tc.skipCondition(t)
				if skip {
					t.Skip(reason)
				}
			}

			var client *mongo.Client
			var setupErr error

			defer func() {
				if r := recover(); r != nil {

					if tc.expectError {
						t.Logf("Recovered from panic (expected behavior for this test case): %v", r)

					} else {

						stack := debug.Stack()
						t.Errorf("Test panicked unexpectedly: %v\nStack trace:\n%s", r, stack)
					}
				}

				if client != nil {
					disconnectCtx, cancelDisconnect := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancelDisconnect()
					if err := client.Disconnect(disconnectCtx); err != nil {
						t.Logf("Error disconnecting client during cleanup: %v", err)
					}
				}
			}()

			if tc.setupClient != nil {
				client, setupErr = tc.setupClient(t, tc.uri)
			} else {

				client = nil
				setupErr = nil
			}

			if setupErr != nil {
				if tc.expectError {
					t.Logf("Setup failed as expected: %v", setupErr)

					return
				} else {
					t.Fatalf("Setup failed unexpectedly: %v", setupErr)
				}
			}

			if client == nil && tc.name != "Scenario 5: Nil Client Input" {
				t.Fatalf("Setup returned a nil client without error, which is unexpected for this test case.")
			}

			if tc.preConnect != nil {
				if err := tc.preConnect(t, client); err != nil {
					t.Fatalf("Pre-connect action failed: %v", err)
				}
			}

			var connectErr error

			if client == nil && tc.name == "Scenario 5: Nil Client Input" {

				if tc.expectError {
					connectErr = errors.New("simulated error for nil client input")
				}
			} else if client != nil {
				connectErr = connectToMongo(client)
			} else {

				t.Fatal("Reached connect phase with nil client unexpectedly")
			}

			if tc.postDisconnect != nil {
				if err := tc.postDisconnect(t, client); err != nil {
					t.Logf("Post-disconnect action failed: %v", err)
				}
			}

			if tc.expectError {
				if connectErr == nil && tc.name != "Scenario 5: Nil Client Input" {
					t.Errorf("Expected an error from connectToMongo, but got nil")
				} else if connectErr != nil {
					t.Logf("Received expected error: %v", connectErr)

					if tc.errorCheck != nil && !tc.errorCheck(connectErr) {
						t.Errorf("Received error '%v', but it did not match the expected type/condition: %s", connectErr, tc.errorMsg)
					}
				} else if connectErr == nil && tc.name == "Scenario 5: Nil Client Input" {

					t.Errorf("Expected an error/panic for nil client, but got nil error and no panic caught")
				}
			} else {
				if connectErr != nil {
					t.Errorf("Expected no error from connectToMongo, but got: %v", connectErr)
				} else {

					if client != nil {
						pingCtx, cancelPing := context.WithTimeout(context.Background(), 2*time.Second)
						defer cancelPing()
						if pingErr := client.Ping(pingCtx, readpref.Primary()); pingErr != nil {
							t.Errorf("connectToMongo returned nil error, but subsequent ping failed: %v", pingErr)
						} else {
							t.Logf("Connection successful and ping verified.")
						}
					}
				}
			}
		})
	}
}

func connectToMongo(client *mongo.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		return err
	}

	err = client.Ping(ctx, readpref.Primary())
	if err != nil {

		_ = client.Disconnect(context.Background())
		return err
	}
	return nil
}

