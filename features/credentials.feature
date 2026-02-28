Feature: Vault and item management
  As an admin operator
  I need to create vaults and store items
  So that TEE instances can later fetch secrets

  Scenario: Create vault and store items via PUT
    Given the server is running
    When I create a vault "my-vault" with name "My Vault"
    Then the response status should be 201
    When I PUT items for vault "my-vault" section "alice@example.com" with fields:
      | key           | value              |
      | refresh_token | rt-alice-secret    |
      | api_key       | key-123            |
    Then the response status should be 200
    And the response JSON "status" should be "updated"

  Scenario: Listing items from nonexistent vault returns empty
    Given the server is running
    When I GET items for vault "nonexistent"
    Then the response status should be 200
