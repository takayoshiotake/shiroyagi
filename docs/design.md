# Design

## Purpose

Shiroyagi is an email client designed for everyday use.

The interface exists to quietly support reading, searching, and organizing
email.

The interface is not the focus—the user's content and task at hand are.

The goal is for users to reach the information and actions they need without
consciously thinking about the UI.

These guidelines are intentionally small. Add a more detailed design system
only when repeated implementation needs justify it.

## Design Philosophy

### A Tool, Not a Showcase

Shiroyagi is a tool.

It is designed to be used, not admired.

Prioritize readability, clarity, and reliability over visual novelty. The
interface should become familiar through repeated use rather than impress at
first sight.

When in doubt, choose the simpler solution.

### Quietness

The interface should not compete with the content.

Colors, movement, and decoration should remain restrained. Only information
that genuinely requires attention should stand out.

Users should notice the content and actions relevant to their current task
before they notice the interface.

### Clarity

Users should naturally understand:

- What is important.
- What can be interacted with.
- What state the application is in.

Hierarchy should come from layout before decoration.

Reducing visual elements is not the goal; making information easier to
understand is.

### Predictability

Similar things should behave similarly. The same action should always produce
the same result.

Avoid surprising interactions that require users to learn special rules. Favor
familiar conventions over novelty.

### Consistency

Every screen should feel like part of the same application.

Prefer extending existing patterns instead of inventing new ones. Before
introducing a new visual style or interaction, consider whether an existing
pattern already solves the problem.

Consistency is more valuable than originality.

## Decision Principles

When multiple designs are possible, prefer the one that is:

1. Easier to read.
2. Easier to understand.
3. More predictable.
4. More comfortable for long-term use.
5. More visually pleasing.

Favor quietness over attention. Favor clarity over decoration. Favor familiarity
over novelty.

When in doubt, choose the design that helps users focus on the content and task
at hand, not the interface.

## Frontend Stack

- Render HTML on the server with Go `html/template`.
- Use htmx for partial page updates.
- Use plain CSS from `static/css/app.css`.
- Avoid a frontend build step or framework unless there is a strong reason and
  the change has been discussed first.
- Avoid custom JavaScript when server-rendered HTML or htmx can provide the same
  behavior.

## Implementation Principles

- Keep navigation, labels, actions, and feedback consistent across pages.
- Make the primary action and current location easy to identify.
- Preserve usable HTML without JavaScript; enhancements must not hide essential
  content or actions.
- Use semantic HTML and ensure controls are usable with a keyboard and have a
  visible focus state.

## Layout

### Standard Pages

Standard pages use the shared application layout and contain, in order:

1. A page header with the page title and optional primary action.
2. The main content area.
3. Secondary or destructive actions near the content they affect.

Keep the content width readable and spacing consistent. Do not duplicate global
navigation or page-level spacing in individual templates.

### Mail View

The mail screen uses a three-pane layout:

1. **Mailboxes:** mailbox navigation. Initially this contains only `INBOX`.
2. **Message list:** the latest 100 messages in the selected mailbox.
3. **Message:** the selected message, including its header, body, and available
   actions.

On narrow screens, show these panes as separate views rather than compressing
all three into unusable columns. Preserve the mailbox and message selection when
navigating between views where practical.

Each pane must handle its own loading, empty, and error state. A missing message
selection is an empty state, not an application error.

## Components

### Forms

- Place a visible label above each input and associate it with the input using
  `for` and `id`.
- Group related fields and keep their order consistent with the task.
- Mark optional fields explicitly; do not rely only on an asterisk to convey
  required fields.
- Put help text and validation errors next to the field they describe.
- Preserve safe user-entered values after validation fails. Never redisplay
  passwords, tokens, or other secrets.
- Disable submission only while a request is in progress, and show that progress
  without changing the button's meaning.

### Buttons and Links

- Use a primary button for the main action in a view. Normally, a view has only
  one primary action.
- Use secondary buttons for supporting actions and danger buttons for
  destructive actions.
- Use links for navigation and buttons for actions.
- Use clear verb-based labels such as `Save account`, `Send`, or `Delete` rather
  than ambiguous labels such as `OK`.
- Keep related actions together. Place table or list-row actions on the right.
- Require confirmation for destructive actions that cannot be easily undone.

### Lists and Tables

- Use a table for data that benefits from aligned columns; use a list when each
  item is primarily a single selectable summary.
- Make the selected message visually distinct and expose its selected state to
  assistive technology.
- Keep the primary identifying content on the left and actions on the right.
- Provide an explicit empty state that explains what is missing and, when
  possible, what the user can do next.
- On narrow screens, hide or stack secondary metadata before truncating primary
  content such as the sender or subject.

### Errors and Feedback

- Show field validation errors beside the relevant input.
- Show request or page-level errors near the content or action that failed.
- Explain what happened and what the user can do next; do not expose internal
  errors, stack traces, secrets, or full JWTs.
- Do not communicate status by color alone. Use concise text and appropriate
  semantic markup, such as an alert region for an error inserted by htmx.
- Keep existing content visible when a partial update fails whenever possible.
- Confirm successful actions when the result is not otherwise obvious.

## CSS Policy

- Put application styles in `static/css/app.css`; avoid inline styles.
- Reuse existing layout and component classes before adding new ones.
- Name classes by component or role rather than by visual appearance.
- Prefer a small set of reusable values for spacing, color, typography, borders,
  and focus states. Introduce CSS custom properties when values begin to repeat.
- Do not make templates depend on a specific CSS framework.

## htmx Policy

- Return server-rendered HTML fragments for partial updates.
- Make the update target and swap behavior explicit at the initiating control or
  its nearest reusable container.
- Include loading, success, empty, and error behavior when designing an
  interaction.
- Keep normal URLs and server routes usable for initial loads, refreshes, and
  direct navigation.
- Move repeated fragment markup into shared templates instead of duplicating it.

## Guidance for UI Changes

When implementing a UI change, including the mail UI work in issue #13:

- Start from the shared application layout.
- Reuse the form, button, list, and error patterns in this document.
- Verify the loading, empty, selected, success, and error states that apply.
- Check both wide and narrow layouts and keyboard navigation.
- Keep application CSS in `static/css/app.css` and avoid introducing a frontend
  framework or build step without discussion.
- Update this document when a new reusable pattern is introduced.
