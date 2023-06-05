/// <reference types="cypress" />
export {}

function login(username:string, password:string) {
    cy.session(
        username,
        () => {
            cy.visit('/login')
            cy.get("button[id=internal]").then((btn) => {
                if (btn.parent().css('display') !== 'none') {
                    btn.trigger('click')
                }
            })
            cy.get('input[id=username]').type(username)
            cy.get('input[id=password]').type(`${password}`)
            cy.get('form').submit()
            cy.url().should('equal', 'http://localhost:8081/')
            cy.get('p[id=greeting]').should('contain', username)

        },
        {
            validate: () => {
                cy.getCookie('jwt').should('exist')
            },
        }
    )
}

declare global {
    namespace Cypress {
        interface Chainable {
            login: typeof login;
        }
    }
}

Cypress.Commands.add('login', login);
