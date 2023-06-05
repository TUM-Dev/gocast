describe('The Login Page', () => {
  it('sets auth cookie when logging in via form submission', function () {
    cy.login("admin", "password")
  })
})
