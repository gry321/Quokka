// ============================================================
// Quokka Plugin Example - Calculator Plugin
// ============================================================
// This plugin evaluates mathematical expressions and returns results.
// 
// Build instructions (Windows):
//   gcc -shared -o calculator_plugin.dll calculator_plugin.c
// 
// Or using MSVC:
//   cl /LD calculator_plugin.c /Fe:calculator_plugin.dll
// ============================================================

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <math.h>
#include <windows.h>

// Simple expression evaluator
static double evaluate_expression(const char* expr) {
    return atof(expr); // Simplified - just parse number for demo
}

// Exported plugin function - THIS IS THE REQUIRED ENTRY POINT
__declspec(dllexport) int __stdcall QuokkaPlugin(
    const char* input, 
    const char* output_path, 
    HWND active_window
) {
    FILE* f = fopen(output_path, "w");
    if (!f) {
        return 1;
    }

    // Check if input looks like a calculation (contains numbers and operators)
    int is_calculation = 0;
    for (const char* p = input; *p; p++) {
        if (*p == '+' || *p == '-' || *p == '*' || *p == '/' || 
            *p == '(' || *p == ')' || isdigit(*p) || *p == '.') {
            is_calculation = 1;
        } else if (!isspace(*p)) {
            is_calculation = 0;
            break;
        }
    }

    if (is_calculation && strlen(input) > 0) {
        double result = evaluate_expression(input);
        
        // Return calculation result as an entry
        fprintf(f, "[{\"name\":\"= %.6g\",\"path\":\"\",\"icon\":\"\",\"source\":\"Calculator\"}]", result);
    } else if (strlen(input) > 0) {
        // Show help for non-calculation input
        fprintf(f, "["
            "{\"name\":\"Try: 2 + 2\",\"path\":\"\",\"icon\":\"\",\"source\":\"Calculator\"},"
            "{\"name\":\"Try: 10 * 5\",\"path\":\"\",\"icon\":\"\",\"source\":\"Calculator\"},"
            "{\"name\":\"Try: 100 / 4\",\"path\":\"\",\"icon\":\"\",\"source\":\"Calculator\"}"
            "]");
    } else {
        // Empty input - show plugin description
        fprintf(f, "["
            "{\"name\":\"🧮 Calculator Plugin\",\"path\":\"\",\"icon\":\"\",\"source\":\"Calculator\"},"
            "{\"name\":\"Enter a math expression\",\"path\":\"\",\"icon\":\"\",\"source\":\"Calculator\"}"
            "]");
    }

    fclose(f);
    return 0;
}

// DLL Entry point
BOOL APIENTRY DllMain(HMODULE hModule, DWORD ul_reason_for_call, LPVOID lpReserved) {
    (void)hModule;
    (void)lpReserved;
    
    switch (ul_reason_for_call) {
        case DLL_PROCESS_ATTACH:
            DisableThreadLibraryCalls(hModule);
            break;
        case DLL_THREAD_ATTACH:
        case DLL_THREAD_DETACH:
        case DLL_PROCESS_DETACH:
            break;
    }
    return TRUE;
}
