@smoke
# don't change first line of this file - the tag is used for the test scripts to identify the test suite

Feature: Kiali login

  User wants to login to Kiali and see landing page

  Background:
    Given all sessions are cleared
    And user opens base url

  Scenario: Try to log in without filling the username and password
    And user clicks my_htpasswd_provider
    And user does not fill in username and password
    Then user sees the "Login is required. Please try again." phrase displayed

  Scenario: Try to log in with an invalid username
    And user clicks my_htpasswd_provider
    And user fills in an invalid username
    Then user sees the "Invalid login or password. Please try again." phrase displayed

  Scenario: Try to log in with an invalid password
    And user clicks my_htpasswd_provider
    And user fills in an invalid password
    Then user sees the "Invalid login or password. Please try again." phrase displayed

  Scenario: Try to log in with a valid password
    And user clicks my_htpasswd_provider
    And user fills in a valid password
    Then user sees the Overview page

  @openshift
  Scenario: Openshift login shows error message when code exchange fails
    And the server will return a login error
    And user clicks my_htpasswd_provider
    And user fills in a valid password
    Then user sees an error message on the login form
    And the error description is in the url
