# Quokka Plugin Development Guide

## Overview

Quokka plugins are native DLL files that extend the launcher's functionality. They can provide custom search results, quick actions, or integrate with external services.

## Plugin Contract

Your DLL must export exactly one function with this signature:

```c
int QuokkaPlugin(const char* input, const char* output_path, HWND active_window);
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `input` | `const char*` | Current search query text (UTF-8, null-terminated) |
| `output_path` | `const char*` | Path to a temporary JSON file where you write results |
| `active_window` | `HWND` | Handle of the currently active (foreground) window |

### Return Value

- `0` = Success
- Non-zero = Error (results will be ignored)

## Output Format

Write a JSON array to the `output_path` file:

```json
[
  {
    "name": "Entry Display Name",
    "path": "C:\\path\\to\\launch\\or\\empty",
    "icon": "",
    "source": "YourPluginName"
  }
]
```

### Entry Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Display name shown in results |
| `path` | string | No | Path to launch when selected (empty for info-only entries) |
| `icon` | string | No | Base64 data URI for custom icon (leave empty for default) |
| `source` | string | No | Plugin identifier (shown as tag in UI) |

## Example Plugin (Calculator)

```c
#include <stdio.h>
#include <windows.h>

__declspec(dllexport) int __stdcall QuokkaPlugin(
    const char* input, 
    const char* output_path, 
    HWND active_window
) {
    FILE* f = fopen(output_path, "w");
    if (!f) return 1;

    // Simple example: always return same result
    fprintf(f, "[{\"name\":\"Hello from plugin\",\"path\":\"\",\"icon\":\"\",\"source\":\"MyPlugin\"}]");
    
    fclose(f);
    return 0;
}

BOOL APIENTRY DllMain(HMODULE hModule, DWORD ul_reason_for_call, LPVOID lpReserved) {
    switch (ul_reason_for_call) {
        case DLL_PROCESS_ATTACH:
            DisableThreadLibraryCalls(hModule);
            break;
    }
    return TRUE;
}
```

## Building Plugins

### Using GCC (MinGW)

```bash
gcc -shared -o myplugin.dll myplugin.c
```

### Using MSVC

```bash
cl /LD myplugin.c /Fe:myplugin.dll
```

### Using CMake

```cmake
cmake_minimum_required(VERSION 3.15)
project(MyPlugin)

add_library(myplugin SHARED myplugin.c)
target_link_libraries(myplugin user32)
set_target_properties(myplugin PROPERTIES PREFIX "")
```

## Installing Plugins

1. **Drag & Drop**: Drag the `.dll` file onto the Quokka search box
2. **Plugin Manager**: Click the plugins icon (⊞) → drop zone

## Best Practices

### Performance
- Keep execution under 100ms
- Use async I/O if calling external APIs
- Cache results when possible

### Security
- Validate all input
- Don't execute arbitrary commands from untrusted sources
- Handle errors gracefully

### User Experience
- Return relevant results based on `input`
- Provide helpful hints when input is empty
- Use emojis in names for visual clarity (optional)

## Advanced Examples

See the `example_plugins/` directory for complete examples:
- `calculator_plugin.c` - Math expression evaluator
- `quickactions_plugin.c` - Quick system actions

## Debugging

Plugins run in the Quokka process. To debug:
1. Attach debugger to `Quokka.exe`
2. Set breakpoints in your DLL
3. Trigger plugin via search

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Plugin not loading | Check exported function name matches exactly |
| No results shown | Verify JSON output is valid |
| Crash on load | Ensure calling convention is `__stdcall` |
| Unicode issues | Use UTF-8 encoding for all strings |

## License

Plugins are loaded dynamically. Ensure your DLL doesn't violate any licenses.
