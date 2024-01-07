@app-details-multi-cluster
# don't change first line of this file - the tag is used for the test scripts to identify the test suite
@multi-cluster
Feature: Kiali App Details page minigraph in multicluster setup

  Some App Details minigraph tests, which required a different setup. 
  Minigraph should not be displayed for an app, if it is not present in the cluster.
  We should also be able to navigate to a remote cluster, if an app of one is present on the graph.
  
  Background:
    Given user is at administrator perspective

  Scenario: Minigraph should not be visible for app, which is not deployed in specific cluster.
    And user is at the details page for the "app" "bookinfo/details" located in the "west" cluster
    Then user does not see a minigraph

  Scenario Outline: User should be able to navigate through the graph to remotely located apps, services and workloads
    Given user is at the details page for the "app" "bookinfo/productpage" located in the "east" cluster
    And the "<name>" "<type>" from the "west" cluster is visible in the minigraph
    When user clicks on the "<name>" "<type>" from the "west" cluster in the graph
    Then the browser is at the details page for the "<type>" "bookinfo/<name>" located in the "west" cluster

    Examples:
      | type     | name       |
      | app      | reviews    |
      | service  | reviews    |
      | workload | reviews-v3 |

  #this is a regression to this bug (https://github.com/kiali/kiali/issues/6185)
  #I used the sleep namespace in the Gherkin, because I feel like we might need a new demoapp for this scenario,
  #if we don't want to change access to bookinfo namespace in the middle of the test run.
  @skip
  Scenario: Remote nodes should be restricted if user does not have access rights to a remote namespace
    When user "is" given access rights to a "sleep" namespace located in the "east" cluster  
    And user "is not" given access rights to a "sleep" namespace located in the "west" cluster  
    And user is at the details page for the "app" "sleep/east" located in the "east" cluster
    Then the nodes located in the "west" cluster should be restricted