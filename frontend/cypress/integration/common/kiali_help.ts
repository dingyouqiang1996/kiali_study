import { When, Then, And, DataTable } from '@badeball/cypress-cucumber-preprocessor';

Then('user can see all of the Help dropdown options', (options: DataTable) => {
  const names = options.raw()[0];
  names.forEach(function (value) {
    cy.get('li[role="none"]').contains(value).should('be.visible');
  });
});

When('user clicks on the {string} button', (title: string) => {
  cy.get('li[role="none"]').contains(title).click();
});

Then('user sees the {string} modal', (title: string) => {
  cy.get('h1.pf-v5-c-modal-box__title').contains(title).should('be.visible');
});

And('user sees information about {int} clusters', (numOfClusters: number) => {
  cy.get('td[data-label="Configuration"]')
    .contains('clusters')
    .parent()
    .find('td[data-label="Value"]')
    .then($td => {
      expect(Object.keys(JSON.parse($td.get(0).innerText)).length).to.eq(numOfClusters);
    });
});
