# VHS tape for the payk demo GIF.
# Render: go build -o /tmp/payk-demo-bin ./cmd/payk, prepare a clean demo
# workspace in /tmp/payk-demo (examples collection), then: vhs docs/demo.tape

Output docs/demo.gif

Set FontSize 15
Set Width 1480
Set Height 780
Set Theme "Catppuccin Mocha"
Set TypingSpeed 80ms
Set Padding 12

Hide
Type "cd /tmp/payk-demo && clear && /tmp/payk-demo-bin"
Enter
Sleep 1.5s
Show

# Browse: expand the users folder, open a request.
Sleep 1s
Type "jj"
Sleep 600ms
Enter
Sleep 800ms
Type "jjj"
Sleep 600ms
Enter
Sleep 1.2s

# Send it.
Space
Sleep 2.5s

# Inspect the response: headers, timings, history, back to body.
Type "ll"
Sleep 600ms
Type "]"
Sleep 1.4s
Type "]"
Sleep 1.6s
Type "]"
Sleep 1.2s
Type "["
Type "["
Type "["
Sleep 800ms

# Search inside the body.
Type "/"
Sleep 300ms
Type "name"
Sleep 900ms
Enter
Sleep 700ms
Type "n"
Sleep 700ms
Type "n"
Sleep 900ms

# Fullscreen the response, then back.
Type "z"
Sleep 1.6s
Type "z"
Sleep 900ms

# Help overlay.
Type "?"
Sleep 2.2s
Escape
Sleep 600ms

Type "q"
Sleep 800ms
