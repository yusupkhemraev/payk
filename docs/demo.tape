# VHS tape for the payk demo GIF.
# Setup before rendering:
#   go build -o /tmp/payk-demo-bin ./cmd/payk
#   mkdir -p /tmp/payk-demo/.payk/collections
#   cp -r .payk/collections/examples /tmp/payk-demo/.payk/collections/
#   cp .payk/environments.yaml /tmp/payk-demo/.payk/
#   cp internal/importer/openapi/testdata/petstore-3.1.yaml /tmp/payk-demo/openapi.yaml
# Then: vhs docs/demo.tape

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

# Browse: expand the users folder, open a request, send it.
Sleep 1s
Type "jj"
Sleep 500ms
Enter
Sleep 700ms
Type "jjj"
Sleep 500ms
Enter
Sleep 1s
Space
Sleep 2.4s

# Response tour: headers, timings, history, back to body.
Type "ll"
Sleep 500ms
Type "]"
Sleep 1.3s
Type "]"
Sleep 1.4s
Type "]"
Sleep 1.3s
Type "["
Type "["
Type "["
Sleep 700ms

# Search the body, then raw bytes toggle.
Type "/"
Sleep 300ms
Type "name"
Sleep 800ms
Enter
Sleep 600ms
Type "n"
Sleep 800ms
Type "r"
Sleep 1.2s
Type "r"
Sleep 700ms

# Fullscreen and back.
Type "z"
Sleep 1.4s
Type "z"
Sleep 700ms

# Create a request right from the tree: a, name, enter — editor opens.
Type "hh"
Sleep 500ms
Type "a"
Sleep 600ms
Type "get todo"
Sleep 500ms
Enter
Sleep 1.2s
Type "j"
Sleep 400ms
Type "i"
Sleep 400ms
Type@25ms "https://jsonplaceholder.typicode.com/todos/1"
Sleep 500ms
Escape
Sleep 500ms
Space
Sleep 2.2s

# Command line completion: commands, then env candidates.
Type ":"
Sleep 1.4s
Type "env "
Sleep 1.6s
Escape
Sleep 500ms

# Import an OpenAPI spec: the whole API lands in the tree.
Type ":"
Sleep 400ms
Type@35ms "import openapi.yaml"
Sleep 600ms
Enter
Sleep 2.2s

# Help overlay.
Type "?"
Sleep 2s
Escape
Sleep 500ms

Type "q"
Sleep 800ms
