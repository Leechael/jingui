Feature: Credential management
  As an admin operator
  I need to register apps and store credentials
  So that TEE instances can later fetch secrets

  Scenario: Register app and store credentials via PUT
    Given the server is running
    When I register an app "desktop-app" of type "gmail" with credentials:
      """
      {"installed":{"client_id":"cid-123","client_secret":"csec-456","redirect_uris":["http://localhost"]}}
      """
    Then the response status should be 201
    When I PUT credentials for app "desktop-app" with user "alice@example.com" and secrets:
      | key           | value              |
      | refresh_token | rt-alice-secret    |
    Then the response status should be 200
    And the response JSON "status" should be "stored"

  Scenario: Reject device flow for missing app
    Given the server is running
    When I POST to "/v1/credentials/device/nonexistent-app"
    Then the response status should be 404
