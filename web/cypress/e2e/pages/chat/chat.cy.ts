describe('The Chat', () => {
    it('Shows the chat', function () {
        cy.login("admin", "password")
        cy.visit('/w/slug/1')
        cy.get('button[id=chat-toggle-button]').should('exist')
        cy.get('button[id=chat-toggle-button]').click()
        cy.get('div[id=chatWrapper]').should('be.visible')
        cy.get('input[id=chatInput]').type('Hello World{enter}')
        cy.wait(200)
    })
})
