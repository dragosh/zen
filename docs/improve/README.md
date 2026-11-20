# Issues and improvements discoverd during human testing

- Look at each individual file and use the `/add-issue` or `/add-spec` to record it and label it corectly (engine, kit, plugin, etc.).
- The title should explain the type
- Once it's registered in github delete the file from here.
- In Claude Code, `/check-feedback` does all of this: it pulls new notes, checks them, files them with `/add-issue` after one confirmation and deletes the filed notes.

## File legend:
- shell : Is the empty shell application just the core functionality
  - Intro view
  - Second view 
  - Panels view
  - Native menu
  - Tray menu
