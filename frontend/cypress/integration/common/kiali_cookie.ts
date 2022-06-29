import { And, Given } from "cypress-cucumber-preprocessor/steps";

const USERNAME = Cypress.env('USERNAME') || 'jenkins';
const PASSWD = Cypress.env('PASSWD')
const AUTH_PROVIDER = Cypress.env('AUTH_PROVIDER') || 'my_htpasswd_provider';

Given('user is at administrator perspective', () => {
    Cypress.Cookies.defaults({
        preserve: 'kiali-token-aes',
    })
    cy.login(AUTH_PROVIDER, USERNAME, PASSWD)
})

And('user visits base url', () => {
    cy.visit('/')
})