# Floating UI in the native launcher

The launcher reuses MyGo's `TooltipBase`, `PopoverBase`, `SelectBase`,
`ComboboxBase` and `DialogBase`, plus the existing surface, button, motion and
notice styles. `surfacePanel` renders the same content in a modal, an edge drawer
or the main page slot; it does not introduce another visual theme.

| Surface | Uses |
| --- | --- |
| Tooltip | Icon help, disabled reasons, truncated text, fixed values and resource summaries |
| Popover | Account switcher, selectors and actionable map details |
| Popconfirm | One account, one world-backup file, an unshared managed Java installation |
| Dialog | Short forms, update-plan approval, rollback, force-stop, shared Java removal and draft-discard decisions |
| Drawer | Account management, game preferences, export options, resource versions/dependencies and read-only pack reports |
| Page | Minecraft/new-modpack installation, all-game logs, Java discovery and required launcher updates |

The 3D structure viewer docks to the map's right edge, remains resizable and can
expand into its existing focused view. Escape restores the docked view first
when expanded; closing the viewer returns keyboard focus to the map.

## Confirmation and focus

`confirmations.go` contains the shared anchored and modal confirmations. Escape,
outside press and Cancel dismiss a popconfirm and restore its trigger. Modal
confirmations use the toolkit focus scope. Buttons name the action. Force-stop
captures a session ID; when several games share a directory, the dialog lets
the user choose the exact session. A stale session cannot fall back to another
running game.

The dialog's Enter shortcut is scoped to its initial panel and text fields;
focused action buttons retain their own keyboard activation. The key recorder
continues to own Escape and Enter while recording.

## Undo and recovery

Enabling/disabling an add-on is immediate. Removing an instance registration,
mod, resource pack, shader pack or saved authentication server offers Undo.
Opaque removal receipts retain the original instance configuration or exact
file locations. Restoring refuses to overwrite a newly registered instance,
same-name file or replacement authentication-server configuration.

Undo notices remain available until used, dismissed or replaced by another
notice. Within a modal, the same notice component is rendered inside its focus
scope so the action remains keyboard accessible. Ordinary information notices
retain their existing lifetime.

Restoring a world or saved game settings uses the existing automatic
pre-restore backup as the recovery path. A settings restore with an edited or
invalid draft asks before discarding that draft.

## Drafts

Dialog and page forms guard user-initiated close, backdrop dismissal, Escape, relevant
navigation, dropping a new task, reloading settings and window close/restart.
Instance settings retain their draft and baseline while a secondary editor is
open. Saving successfully closes through the existing completion path without
asking to discard saved values. Auto-saved launcher settings do not get a
discard prompt.

## Verification

The native UI tests exercise focus return, single execution, keyboard Undo
inside a modal, draft preservation, viewport bounds and the existing map and
installation flows. Service tests cover exact-version restoration, instance
configuration persistence and refusal to overwrite replacement data.

Run the native UI and catalog suites with:

```sh
go test -tags production,mygo_noinspector ./internal/nativeui ./internal/i18n
go test ./internal/launcher -run 'Test(ModUndo|RootUndo|AssetAndAuthServerUndo)'
```
