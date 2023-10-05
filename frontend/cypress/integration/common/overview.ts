import { Before, Given, Then, When, And } from '@badeball/cypress-cucumber-preprocessor';
import { ensureKialiFinishedLoading } from './transition';


const CLUSTER1_CONTEXT = Cypress.env('CLUSTER1_CONTEXT')
const CLUSTER2_CONTEXT = Cypress.env('CLUSTER2_CONTEXT')

Before(() => {
  // Focing to not stop cypress on unexpected errors not related to the tests.
  // There are some random failures due timeouts/loadtime/framework that throws some error in the browser.
  // After reviewing the tests failures, those are unrelated to the app, so,
  // it needs this event to not fail the CI action due some "slow" action or similar.
  // This is something to review in future iterations when tests are solid, but I haven't found a better way to
  // solve this issue.
  cy.on('uncaught:exception', (err, runnable, promise) => {
    // when the exception originated from an unhandled promise
    // rejection, the promise is provided as a third argument
    // you can turn off failing the test in this case
    if (promise) {
      return false;
    }
    // we still want to ensure there are no other unexpected
    // errors, so we let them fail the test
  });
});

Given('a healthy application in the cluster', function () {
  this.targetNamespace = 'bookinfo';
  this.targetApp = 'productpage';
});

Given('a healthy application in the remote cluster', function () {
  this.targetNamespace = 'bookinfo';
  this.targetApp = 'ratings';
});

Given('an idle application in the cluster', function () {
  this.targetNamespace = 'sleep';
  this.targetApp = 'sleep';

  cy.exec('kubectl scale -n sleep --replicas=0 deployment/sleep');
});

Given('an idle application in the remote cluster', function () {
  this.targetNamespace = 'bookinfo';
  this.targetApp = 'reviews';
});

Given('a failing application in the mesh', function () {
  this.targetNamespace = 'alpha';
  this.targetApp = 'v-server';
});

Given('a degraded application in the mesh', function () {
  this.targetNamespace = 'alpha';
  this.targetApp = 'b-client';
});

When('user clicks in the {string} view', view => {
  cy.get('button[data-test="overview-type-' + view + '"]')
    .click()
    // Using the #loading_kiali_spinner selector we can control when the UI is still loading some data
    // That may prevent that the test progress in cases where we need more control.
    .get('#loading_kiali_spinner')
    .should('not.exist');
});

When(`user filters {string} namespace`, (ns: string) => {
  cy.get('select[aria-label="filter_select_type"]').select('Namespace').should('have.value', 'Namespace');
  cy.get('input[aria-label="filter_input_value"]')
    .type(ns)
    .type('{enter}')
    .get('#loading_kiali_spinner')
    .should('not.exist');
});

When(`user filters {string} health`, (health: string) => {
  cy.get('select[aria-label="filter_select_type"]').select('Health').should('have.value', 'Health');
  cy.get('select[aria-label="filter_select_value"]').select(health).get('#loading_kiali_spinner').should('not.exist');
});

When(`user selects Health for {string}`, type => {
  let innerId = '';
  switch (type) {
    case 'Apps':
      innerId = 'app';
      break;
    case 'Workloads':
      innerId = 'workload';
      break;
    case 'Services':
      innerId = 'service';
      break;
  }
  cy.get('button#overview-type-toggle').click().get('#loading_kiali_spinner').should('not.exist');
  cy.get(`button[id="${innerId}"]`).click().get('#loading_kiali_spinner').should('not.exist');
});

When(`user sorts by name desc`, () => {
  cy.get('button[data-sort-asc="true"]').click().get('#loading_kiali_spinner').should('not.exist');
});

When(`user selects {string} time range`, interval => {
  let innerId = '';
  switch (interval) {
    case 'Last 10m':
      innerId = '600';
      break;
  }
  cy.get('button#time_range_duration-toggle').click().get('#loading_kiali_spinner').should('not.exist');
  cy.get(`button[id="${innerId}"]`).click().get('#loading_kiali_spinner').should('not.exist');
});

When(`user selects {string} traffic direction`, direction => {
  let innerId = '';
  switch (direction) {
    case 'Outbound':
      innerId = 'outbound';
      break;
    case 'Inbound':
      innerId = 'inbound';
      break;
  }
  cy.get('button#direction-type-toggle').click().get('#loading_kiali_spinner').should('not.exist');
  cy.get(`button[id="${innerId}"]`).click().get('#loading_kiali_spinner').should('not.exist');
});

When('I fetch the overview of the cluster', function () {
  cy.visit('/console/overview?refresh=0');
});

Then(`user sees the {string} namespace card`, (ns: string) => {
  cy.get(`div[data-test^="${ns}"]`);
});

Then(`user sees the {string} namespace card in cluster {string}`, (ns: string, cluster: string) => {
  // TODO: Incorporate cluster into existing namespace checks with cluster+ns as data-test-id.
  cy.get(`div[data-test^="${ns}"]`).contains(cluster).should('exist').and('length', 1);
});

Then(`user doesn't see the {string} namespace card`, ns => {
  cy.get('div[data-test^="' + ns + '"]').should('not.exist');
});

Then(`user sees a {string} {string} namespace`, (view, ns: string) => {
  if (view === 'LIST') {
    cy.get('td[role="gridcell"]').contains(ns);
  } else {
    cy.get('div[data-test="' + ns + '-' + view + '"]');
  }
});

Then(`user sees the {string} namespace with {string}`, (ns, type) => {
  let innerType = '';
  switch (type) {
    case 'Applications':
      innerType = 'app';
      break;
    case 'Workloads':
      innerType = 'workload';
      break;
    case 'Services':
      innerType = 'service';
      break;
  }
  cy.get('div[data-test^="' + ns + '"]').find('[data-test="overview-type-' + innerType + '"]');
});

Then(`user sees the {string} namespace list`, (nslist: string) => {
  const nss = nslist.split(',');
  cy.get('div[data-ouia-component-type="PF5/Card"]')
    .should('have.length', nss.length)
    .each(($a, i) => {
      expect($a.attr('data-test')).includes(nss[i]);
    });
});

Then(`user sees the {string} namespace with {string} traffic {string}`, (ns, direction, duration) => {
  cy.get('div[data-test^="' + ns + '"]').find('div[data-test="sparkline-' + direction + '-duration-' + duration + '"]');
});

Then('user sees the memory chart', () => {
  cy.get('div[data-test="memory-chart"]').should('exist');
});

And('user sees the cpu chart', () => {
  cy.get('div[data-test="cpu-chart"]').should('exist');
});

Then('there should be a {string} application indicator in the namespace', function (healthStatus: string) {
  cy.get(`[data-test=${this.targetNamespace}-EXPAND] [data-test=overview-app-health]`)
    .find('span')
    .filter(`.icon-${healthStatus}`)
    .should('exist');
});

Then('there should be a {string} application indicator in the namespace in the {string} cluster', function (healthStatus: string, cluster:string) {
  cy.get(
    `[data-test=CardItem_${this.targetNamespace}_${cluster}] [data-test=overview-app-health]`).find('span').filter(`.icon-${healthStatus}`)
  .should('exist');
});

Then('the {string} application indicator should list the application', function (healthStatus: string) {
  let healthIndicatorStatusKey = healthStatus;
  if (healthStatus === 'idle') {
    healthIndicatorStatusKey = 'not-ready';
  }

  cy.get(`[data-test=${this.targetNamespace}-EXPAND] [data-test=overview-app-health]`)
    .find('span')
    .filter(`.icon-${healthStatus}`)
    .trigger('mouseenter');
  cy.get(
    `[aria-label='Overview status'] [data-test=${this.targetNamespace}-${healthIndicatorStatusKey}-${this.targetApp}]`
  )
    .find('span')
    .filter(`.icon-${healthStatus}`)
    .should('exist');
  cy.get(
    `[aria-label='Overview status'] [data-test=${this.targetNamespace}-${healthIndicatorStatusKey}-${this.targetApp}]`
  ).should('contain.text', this.targetApp);
});

Then('the {string} application indicator for the {string} cluster should list the application', function (healthStatus: string, cluster:string) {
  let healthIndicatorStatusKey = healthStatus;
  if (healthStatus === 'idle') {
    healthIndicatorStatusKey = 'not-ready';
  }

  cy.get(
    `[data-test=CardItem_${this.targetNamespace}_${cluster}] [data-test=overview-app-health]`).find('span').filter(`.icon-${healthStatus}`)
  .trigger('mouseenter');
  cy.get(
    `[aria-label='Overview status'] [data-test=${this.targetNamespace}-${healthIndicatorStatusKey}-${this.targetApp}]`).find('span').filter(`.icon-${healthStatus}`)
    .should('exist');
  cy.get(
    `[aria-label='Overview status'] [data-test=${this.targetNamespace}-${healthIndicatorStatusKey}-${this.targetApp}]`
  ).should('contain.text', this.targetApp);
});

// New CP Card validations
When('user hovers over the MinTLS locker', view => {
  cy.get('[data-test="lockerCA"]').should('exist');
});

Then('the toggle on the right side of the {string} namespace card exists', (ns: string) => {
  ensureKialiFinishedLoading();
  cy.get('div[data-test^="' + ns + '"]').should('exist');
});

Then('the user sees the certificates information', view => {
  cy.get('[data-test="lockerCA"]').trigger('mouseenter').get('[role="tooltip"]').contains('Valid From');
});

// We will suppose that the min TLS Version was not set
// So we verify the default
Then('the minimum TLS version', view => {
  cy.get('[data-test="label-TLS"]').contains('N/A');
});

Then('the user sees no information related to canary upgrades', view => {
  cy.get('[data-test="canary-upgrade"]').should('not.exist');
});

Then('the user sees information related to canary upgrades', view => {
  cy.get('[data-test="canary-upgrade"]').should('exist');
});

Then('user sees the {string} cluster badge in the Kiali header', (name:string) =>{
  cy.get('[data-test="cluster-icon"]').contains(name).should('be.visible');
});

And('user sees the {string} label in both {string} namespace cards', (label:string, ns:string) => {
  cy.get(`[data-test="CardItem_${ns}_east"]`).contains(label).should('be.visible');
  cy.get(`[data-test="CardItem_${ns}_west"]`).contains(label).should('be.visible');
})

And('the toggle on the right side of both {string} namespace cards exists', (ns:string) => {
  ensureKialiFinishedLoading();
  cy.get(`[data-test="CardItem_${ns}_east"]`).find('[aria-label="Actions"]').should('exist');
  cy.get(`[data-test="CardItem_${ns}_west"]`).find('[aria-label="Actions"]').should('exist');
});

And('Istio config should not be available for the {string} {string}', (cluster:string, ns:string) => {
  cy.get(`[data-test="CardItem_${ns}_${cluster}"]`).contains('Istio config').siblings().contains('N/A');
}); 

And('health should be different for {string} and {string} {string}', (cluster1:string, cluster2:string, ns:string) => {
  if (ns == 'bookinfo'){
      cy.get(`[data-test="CardItem_${ns}_${cluster1}"]`).find('[data-test="overview-type-app"]').contains(`5 app`);
      cy.get(`[data-test="CardItem_${ns}_${cluster2}"]`).find('[data-test="overview-type-app"]').contains(`4 app`);
  }
  else {
    cy.exec(`kubectl get pods -n ${ns} -l app --context ${CLUSTER1_CONTEXT} --no-headers | wc -l`).then((result) => {
      cy.get(`[data-test="CardItem_${ns}_${cluster1}"]`).find('[data-test="overview-type-app"]').contains(`${result.stdout} app`);
    });
    cy.exec(`kubectl get pods -n ${ns} -l app --context ${CLUSTER2_CONTEXT} --no-headers | wc -l`).then((result) => {
      cy.get(`[data-test="CardItem_${ns}_${cluster2}"]`).find('[data-test="overview-type-app"]').contains(`${result.stdout} app`);
    });
  }
})

And('user sees the {string} label in the {string} namespace card', (label: string, ns: string) => {
  cy.log(label);
  cy.get('div[data-test^="' + ns + '"]')
    .contains(label)
    .should('be.visible');
});

And('user does not see any cluster badge in the {string} namespace card', (ns: string) => {
  cy.get(`[data-test="${ns}-EXPAND"]`).within($card => {
    cy.get('#pfbadge-C').should('not.exist');
  });
});

And('user sees the {string} label in the {string} {string} namespace card',(label:string, cluster:string, ns:string) =>{
  cy.get(`[data-test="CardItem_${ns}_${cluster}"]`).contains(label).should('be.visible');
});

And('user does not see the {string} label in the {string} {string} namespace card',(label:string, cluster:string, ns:string) =>{
  cy.get(`[data-test="CardItem_${ns}_${cluster}"]`).contains(label).should('not.exist');
});
