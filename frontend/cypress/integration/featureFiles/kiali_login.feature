Feature: Kiali login

  User wants to login to Kiali and see landing page
  
  Scenario: Open Kaili home page
    Given user opens base url
    And user clicks Log In With OpenShift
    And user clicks my_htpasswd_provider
    And user fill in username and password
    Then user see console in URL