# Wish Lists design

Wish Lists is a private Tool at `/wish` for one invited family group. It uses [The Lobby's design system](../../DESIGN.md) and [existing sign-in and store](../adr/0019-family-wish-lists-in-the-site-table.md).

## Browse and edit

- Choose a name to see that person's wishes. On a wide screen the family list picker sits beside the selected list. On a phone the picker sits above it.
- Start a personal list by entering the name your family should see. You can rename it and add, edit, or remove wishes. Removing a list asks for confirmation.
- Add each wish with a title and optional link and notes. A wish link opens when you choose it. The site never fetches it for previews.
- The Owner can also create and edit managed lists for people who do not sign in. Another Member's personal list is read-only.

The first view uses the compact Tool header, a Wish Lists heading, and the family picker. Tactile custard marks the main action, outlined controls handle secondary actions, and the shared stepped footer links to Privacy. The family picker keeps the names in view as you switch between lists. The layout avoids side-scrolling on phones. Controls use visible focus and descriptive labels. Save feedback and errors appear beside the action that produced them.

## Access and removal

Lobby Admin awards the `wish` grant to individual Members. A Member who has it can see every list. The page shows a checking state until the site confirms access. It offers sign-in with a return to `/wish` when signed out, a way back to Tools when access is denied, and Retry if the service could not check access. A storage error has its own retry state. The site keeps list names, wishes, and links out of the prerendered page. A tab checks access again when you return to it.

Taking the grant away ends access and leaves the Member's personal list visible to the family. Removing the Member from The Lobby deletes their personal list. Managed lists stay unless the Owner removes them in Wish Lists. Two edits to the same list can conflict. The site keeps the saved version and asks the editor to refresh before trying again.
