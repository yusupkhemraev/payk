# VHS tape for the payk demo GIF.
# Render with: vhs docs/demo.tape  (https://github.com/charmbracelet/vhs)

Output docs/demo.gif

Set FontSize 16
Set Width 1400
Set Height 800
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms

Hide
Type "go run ./cmd/payk"
Enter
Sleep 2s
Show

# Browse the tree: expand a folder, open a request.
Type "j"
Sleep 500ms
Enter
Sleep 800ms
Type "j"
Sleep 300ms
Enter
Sleep 1s

# Send it and inspect the response.
Space
Sleep 2s
Type "ll"
Sleep 500ms
Type "]"
Sleep 1s
Type "]"
Sleep 1.5s
Type "["
Type "["
Sleep 500ms

# Search inside the body.
Type "/"
Sleep 300ms
Type "name"
Sleep 800ms
Enter
Sleep 800ms
Type "n"
Sleep 800ms

# Fullscreen the response, then back.
Type "z"
Sleep 1.5s
Type "z"
Sleep 800ms

# Help overlay.
Type "?"
Sleep 2.5s
Escape
Sleep 500ms

Type "q"
Sleep 1s
