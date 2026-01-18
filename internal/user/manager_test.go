package user

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestNewUserManager_CreatesDefaultUser verifies that when NewUserManager is called
// with an empty data directory, it creates the default "felonius" user.
func TestNewUserManager_CreatesDefaultUser(t *testing.T) {
	temporaryDataDirectory := t.TempDir()

	userManager, err := NewUserManager(temporaryDataDirectory)
	if err != nil {
		t.Fatalf("NewUserManager failed: %v", err)
	}

	// Verify the default felonius user was created
	defaultUser, exists := userManager.GetUser("felonius")
	if !exists {
		t.Fatal("Expected default 'felonius' user to be created, but it was not found")
	}

	// Verify expected properties of the default user
	if defaultUser.Username != "felonius" {
		t.Errorf("Expected username 'felonius', got '%s'", defaultUser.Username)
	}

	if defaultUser.Handle != "Felonius" {
		t.Errorf("Expected handle 'Felonius', got '%s'", defaultUser.Handle)
	}

	if defaultUser.AccessLevel != 10 {
		t.Errorf("Expected access level 10, got %d", defaultUser.AccessLevel)
	}

	if !defaultUser.Validated {
		t.Error("Expected default user to be validated")
	}

	// Verify users.dat file was created
	usersDataFilePath := filepath.Join(temporaryDataDirectory, "users.dat")
	if _, err := os.Stat(usersDataFilePath); os.IsNotExist(err) {
		t.Error("Expected users.dat file to be created")
	}
}

// TestAuthenticate_ValidPassword verifies that Authenticate returns true
// when given a correct username and password combination.
func TestAuthenticate_ValidPassword(t *testing.T) {
	temporaryDataDirectory := t.TempDir()

	userManager, err := NewUserManager(temporaryDataDirectory)
	if err != nil {
		t.Fatalf("NewUserManager failed: %v", err)
	}

	// The default password for felonius is "password"
	authenticatedUser, authenticationSucceeded := userManager.Authenticate("felonius", "password")

	if !authenticationSucceeded {
		t.Fatal("Expected authentication to succeed with correct password")
	}

	if authenticatedUser == nil {
		t.Fatal("Expected authenticated user to be returned, got nil")
	}

	if authenticatedUser.Username != "felonius" {
		t.Errorf("Expected authenticated user username to be 'felonius', got '%s'", authenticatedUser.Username)
	}
}

// TestAuthenticate_ValidPassword_CaseInsensitiveUsername verifies that username
// lookup is case-insensitive during authentication.
func TestAuthenticate_ValidPassword_CaseInsensitiveUsername(t *testing.T) {
	temporaryDataDirectory := t.TempDir()

	userManager, err := NewUserManager(temporaryDataDirectory)
	if err != nil {
		t.Fatalf("NewUserManager failed: %v", err)
	}

	// Try authentication with uppercase username
	authenticatedUser, authenticationSucceeded := userManager.Authenticate("FELONIUS", "password")

	if !authenticationSucceeded {
		t.Fatal("Expected authentication to succeed with uppercase username")
	}

	if authenticatedUser == nil {
		t.Fatal("Expected authenticated user to be returned, got nil")
	}
}

// TestAuthenticate_InvalidPassword verifies that Authenticate returns false
// when given an incorrect password.
func TestAuthenticate_InvalidPassword(t *testing.T) {
	temporaryDataDirectory := t.TempDir()

	userManager, err := NewUserManager(temporaryDataDirectory)
	if err != nil {
		t.Fatalf("NewUserManager failed: %v", err)
	}

	authenticatedUser, authenticationSucceeded := userManager.Authenticate("felonius", "wrongpassword")

	if authenticationSucceeded {
		t.Fatal("Expected authentication to fail with incorrect password")
	}

	if authenticatedUser != nil {
		t.Error("Expected authenticated user to be nil when authentication fails")
	}
}

// TestAuthenticate_UnknownUser verifies that Authenticate returns false
// when given a username that does not exist.
func TestAuthenticate_UnknownUser(t *testing.T) {
	temporaryDataDirectory := t.TempDir()

	userManager, err := NewUserManager(temporaryDataDirectory)
	if err != nil {
		t.Fatalf("NewUserManager failed: %v", err)
	}

	authenticatedUser, authenticationSucceeded := userManager.Authenticate("nonexistentuser", "anypassword")

	if authenticationSucceeded {
		t.Fatal("Expected authentication to fail for unknown user")
	}

	if authenticatedUser != nil {
		t.Error("Expected authenticated user to be nil for unknown user")
	}
}

// TestAddUser_CreatesWithHashedPassword verifies that AddUser creates a new user
// with a properly bcrypt-hashed password (not stored in plaintext).
func TestAddUser_CreatesWithHashedPassword(t *testing.T) {
	temporaryDataDirectory := t.TempDir()

	userManager, err := NewUserManager(temporaryDataDirectory)
	if err != nil {
		t.Fatalf("NewUserManager failed: %v", err)
	}

	const testUsername = "testuser"
	const testPassword = "securePassword123"
	const testHandle = "TestHandle"
	const testRealName = "Test User"
	const testPhoneNumber = "555-1234"
	const testGroupLocation = "Test Group"

	newUser, addUserError := userManager.AddUser(testUsername, testPassword, testHandle, testRealName, testPhoneNumber, testGroupLocation)
	if addUserError != nil {
		t.Fatalf("AddUser failed: %v", addUserError)
	}

	// Verify user was created with correct properties
	if newUser.Username != testUsername {
		t.Errorf("Expected username '%s', got '%s'", testUsername, newUser.Username)
	}

	if newUser.Handle != testHandle {
		t.Errorf("Expected handle '%s', got '%s'", testHandle, newUser.Handle)
	}

	// Verify password is hashed, not plaintext
	if newUser.PasswordHash == testPassword {
		t.Fatal("Password was stored in plaintext, expected it to be hashed")
	}

	if newUser.PasswordHash == "" {
		t.Fatal("Password hash is empty")
	}

	// Verify the hash is valid bcrypt by comparing with the original password
	comparisonError := bcrypt.CompareHashAndPassword([]byte(newUser.PasswordHash), []byte(testPassword))
	if comparisonError != nil {
		t.Errorf("Password hash does not match original password: %v", comparisonError)
	}

	// Verify the user can be retrieved from the manager
	retrievedUser, userExists := userManager.GetUser(testUsername)
	if !userExists {
		t.Fatal("Expected user to exist in manager after AddUser")
	}

	if retrievedUser.Username != testUsername {
		t.Errorf("Retrieved user username mismatch: expected '%s', got '%s'", testUsername, retrievedUser.Username)
	}
}

// TestAddUser_AssignsUniqueUserID verifies that AddUser assigns unique
// sequential user IDs to newly created users.
func TestAddUser_AssignsUniqueUserID(t *testing.T) {
	temporaryDataDirectory := t.TempDir()

	userManager, err := NewUserManager(temporaryDataDirectory)
	if err != nil {
		t.Fatalf("NewUserManager failed: %v", err)
	}

	// Default felonius user should have ID 1
	feloniusUser, _ := userManager.GetUser("felonius")
	if feloniusUser.ID != 1 {
		t.Errorf("Expected felonius user ID to be 1, got %d", feloniusUser.ID)
	}

	// Add first new user - should get ID 2
	firstNewUser, err := userManager.AddUser("user1", "pass1", "Handle1", "User One", "555-0001", "Group1")
	if err != nil {
		t.Fatalf("AddUser failed for user1: %v", err)
	}

	if firstNewUser.ID != 2 {
		t.Errorf("Expected first new user ID to be 2, got %d", firstNewUser.ID)
	}

	// Add second new user - should get ID 3
	secondNewUser, err := userManager.AddUser("user2", "pass2", "Handle2", "User Two", "555-0002", "Group2")
	if err != nil {
		t.Fatalf("AddUser failed for user2: %v", err)
	}

	if secondNewUser.ID != 3 {
		t.Errorf("Expected second new user ID to be 3, got %d", secondNewUser.ID)
	}
}

// TestAddUser_RejectsDuplicateUsername verifies that AddUser returns an error
// when attempting to create a user with an existing username.
func TestAddUser_RejectsDuplicateUsername(t *testing.T) {
	temporaryDataDirectory := t.TempDir()

	userManager, err := NewUserManager(temporaryDataDirectory)
	if err != nil {
		t.Fatalf("NewUserManager failed: %v", err)
	}

	// First user should succeed
	_, addFirstUserError := userManager.AddUser("duplicateuser", "password1", "Handle1", "First User", "555-0001", "Group1")
	if addFirstUserError != nil {
		t.Fatalf("First AddUser failed: %v", addFirstUserError)
	}

	// Second user with same username should fail
	_, addDuplicateUserError := userManager.AddUser("duplicateuser", "password2", "Handle2", "Second User", "555-0002", "Group2")
	if addDuplicateUserError == nil {
		t.Fatal("Expected AddUser to fail for duplicate username, but it succeeded")
	}

	if addDuplicateUserError != ErrUserExists {
		t.Errorf("Expected ErrUserExists error, got: %v", addDuplicateUserError)
	}
}

// TestAddUser_RejectsDuplicateUsername_CaseInsensitive verifies that duplicate
// username checking is case-insensitive.
func TestAddUser_RejectsDuplicateUsername_CaseInsensitive(t *testing.T) {
	temporaryDataDirectory := t.TempDir()

	userManager, err := NewUserManager(temporaryDataDirectory)
	if err != nil {
		t.Fatalf("NewUserManager failed: %v", err)
	}

	// First user with lowercase username
	_, addFirstUserError := userManager.AddUser("sameuser", "password1", "Handle1", "First User", "555-0001", "Group1")
	if addFirstUserError != nil {
		t.Fatalf("First AddUser failed: %v", addFirstUserError)
	}

	// Second user with uppercase version of same username should fail
	_, addDuplicateUserError := userManager.AddUser("SAMEUSER", "password2", "Handle2", "Second User", "555-0002", "Group2")
	if addDuplicateUserError == nil {
		t.Fatal("Expected AddUser to fail for case-insensitive duplicate username")
	}

	if addDuplicateUserError != ErrUserExists {
		t.Errorf("Expected ErrUserExists error, got: %v", addDuplicateUserError)
	}
}

// TestAddUser_RejectsDuplicateHandle verifies that AddUser returns an error
// when attempting to create a user with an existing handle.
func TestAddUser_RejectsDuplicateHandle(t *testing.T) {
	temporaryDataDirectory := t.TempDir()

	userManager, err := NewUserManager(temporaryDataDirectory)
	if err != nil {
		t.Fatalf("NewUserManager failed: %v", err)
	}

	// First user should succeed
	_, addFirstUserError := userManager.AddUser("user1", "password1", "SameHandle", "First User", "555-0001", "Group1")
	if addFirstUserError != nil {
		t.Fatalf("First AddUser failed: %v", addFirstUserError)
	}

	// Second user with same handle should fail
	_, addDuplicateHandleError := userManager.AddUser("user2", "password2", "SameHandle", "Second User", "555-0002", "Group2")
	if addDuplicateHandleError == nil {
		t.Fatal("Expected AddUser to fail for duplicate handle, but it succeeded")
	}

	if addDuplicateHandleError != ErrHandleExists {
		t.Errorf("Expected ErrHandleExists error, got: %v", addDuplicateHandleError)
	}
}

// TestGetUser_ReturnsCorrectUser verifies that GetUser retrieves the correct
// user by username.
func TestGetUser_ReturnsCorrectUser(t *testing.T) {
	temporaryDataDirectory := t.TempDir()

	userManager, err := NewUserManager(temporaryDataDirectory)
	if err != nil {
		t.Fatalf("NewUserManager failed: %v", err)
	}

	const testUsername = "lookupuser"
	const testHandle = "LookupHandle"

	_, addUserError := userManager.AddUser(testUsername, "password", testHandle, "Lookup User", "555-1234", "TestGroup")
	if addUserError != nil {
		t.Fatalf("AddUser failed: %v", addUserError)
	}

	retrievedUser, userExists := userManager.GetUser(testUsername)
	if !userExists {
		t.Fatal("Expected user to exist, but GetUser returned false")
	}

	if retrievedUser.Username != testUsername {
		t.Errorf("Expected username '%s', got '%s'", testUsername, retrievedUser.Username)
	}

	if retrievedUser.Handle != testHandle {
		t.Errorf("Expected handle '%s', got '%s'", testHandle, retrievedUser.Handle)
	}
}

// TestGetUser_NotFound verifies that GetUser returns false for a non-existent user.
func TestGetUser_NotFound(t *testing.T) {
	temporaryDataDirectory := t.TempDir()

	userManager, err := NewUserManager(temporaryDataDirectory)
	if err != nil {
		t.Fatalf("NewUserManager failed: %v", err)
	}

	_, userExists := userManager.GetUser("nonexistentuser")
	if userExists {
		t.Error("Expected GetUser to return false for non-existent user")
	}
}

// TestAuthenticate_IncrementsTimesCalled verifies that successful authentication
// increments the user's TimesCalled counter.
func TestAuthenticate_IncrementsTimesCalled(t *testing.T) {
	temporaryDataDirectory := t.TempDir()

	userManager, err := NewUserManager(temporaryDataDirectory)
	if err != nil {
		t.Fatalf("NewUserManager failed: %v", err)
	}

	// Get initial times called
	userBeforeAuth, _ := userManager.GetUser("felonius")
	initialTimesCalled := userBeforeAuth.TimesCalled

	// Authenticate
	_, authSucceeded := userManager.Authenticate("felonius", "password")
	if !authSucceeded {
		t.Fatal("Authentication should have succeeded")
	}

	// Check times called was incremented
	userAfterAuth, _ := userManager.GetUser("felonius")
	if userAfterAuth.TimesCalled != initialTimesCalled+1 {
		t.Errorf("Expected TimesCalled to be %d, got %d", initialTimesCalled+1, userAfterAuth.TimesCalled)
	}
}

// TestNewUserManager_PersistsAndLoadsUsers verifies that users are persisted
// to disk and can be loaded by a new UserManager instance.
func TestNewUserManager_PersistsAndLoadsUsers(t *testing.T) {
	temporaryDataDirectory := t.TempDir()

	// Create first manager and add a user
	firstManager, err := NewUserManager(temporaryDataDirectory)
	if err != nil {
		t.Fatalf("First NewUserManager failed: %v", err)
	}

	const testUsername = "persisteduser"
	const testHandle = "PersistedHandle"
	const testPassword = "persistedpassword"

	_, addUserError := firstManager.AddUser(testUsername, testPassword, testHandle, "Persisted User", "555-1234", "TestGroup")
	if addUserError != nil {
		t.Fatalf("AddUser failed: %v", addUserError)
	}

	// Create second manager pointing to same data directory - should load the user
	secondManager, err := NewUserManager(temporaryDataDirectory)
	if err != nil {
		t.Fatalf("Second NewUserManager failed: %v", err)
	}

	// Verify the user persisted and can be loaded
	loadedUser, userExists := secondManager.GetUser(testUsername)
	if !userExists {
		t.Fatal("Expected persisted user to be loaded by second manager")
	}

	if loadedUser.Username != testUsername {
		t.Errorf("Expected username '%s', got '%s'", testUsername, loadedUser.Username)
	}

	if loadedUser.Handle != testHandle {
		t.Errorf("Expected handle '%s', got '%s'", testHandle, loadedUser.Handle)
	}

	// Verify the user can authenticate with second manager
	_, authSucceeded := secondManager.Authenticate(testUsername, testPassword)
	if !authSucceeded {
		t.Error("Expected persisted user to authenticate with second manager")
	}
}
