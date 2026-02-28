Feature: Secret fetching via challenge-response
  As a TEE instance
  I need to fetch secrets through a challenge-response protocol
  So that only the holder of the private key can decrypt secrets

  Background:
    Given the server is running
    And a vault "gmail-app" exists with items for "user@example.com":
      | key           | value                |
      | client_id     | test-cid             |
      | client_secret | test-csec            |
      | refresh_token | rt-user-secret-value |
    And a TEE instance is registered with dstack app id "gmail-app"
    And the instance has access to vault "gmail-app"

  Scenario: Full challenge-response flow to fetch secrets
    When I request a challenge for the TEE instance
    And I solve the challenge and fetch secrets:
      | ref                                                |
      | jingui://gmail-app/user@example.com/client_id      |
      | jingui://gmail-app/user@example.com/client_secret   |
      | jingui://gmail-app/user@example.com/refresh_token   |
    Then the response status should be 200
    And the decrypted secret "jingui://gmail-app/user@example.com/client_id" should be "test-cid"
    And the decrypted secret "jingui://gmail-app/user@example.com/client_secret" should be "test-csec"
    And the decrypted secret "jingui://gmail-app/user@example.com/refresh_token" should be "rt-user-secret-value"

  Scenario: Reject fetch with expired/invalid challenge
    When I POST to "/v1/secrets/fetch" with JSON:
      """
      {
        "fid": "{{fid}}",
        "secret_references": ["jingui://gmail-app/user@example.com/client_id"],
        "challenge_id": "bogus-challenge-id",
        "challenge_response": "Ym9ndXMtcmVzcG9uc2U="
      }
      """
    Then the response status should be 401

  Scenario: Reject fetch for vault without access
    When I request a challenge for the TEE instance
    And I solve the challenge and fetch secrets:
      | ref                                                |
      | jingui://other-app/user@example.com/client_id      |
    Then the response status should be 403
