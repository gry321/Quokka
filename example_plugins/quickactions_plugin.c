// ============================================================
// Quokka Plugin Example - Quick Actions Plugin
// ============================================================
// This plugin provides quick system actions like opening folders,
// running commands, or accessing common locations.
// 
// Build instructions (Windows):
//   gcc -shared -o quickactions_plugin.dll quickactions_plugin.c
// ============================================================

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <windows.h>

// Helper: convert string to lowercase
static void to_lower(char* str) {
    for (char* p = str; *p; p++) {
        *p = tolower((unsigned char)*p);
    }
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

    char lower_input[256] = {0};
    strncpy(lower_input, input, sizeof(lower_input) - 1);
    to_lower(lower_input);

    // Check for keywords
    int show_all = (strlen(input) == 0);
    int match_downloads = strstr(lower_input, "download") || show_all;
    int match_documents = strstr(lower_input, "document") || strstr(lower_input, "doc") || show_all;
    int match_desktop = strstr(lower_input, "desktop") || show_all;
    int match_temp = strstr(lower_input, "temp") || show_all;
    int match_cmd = strstr(lower_input, "cmd") || strstr(lower_input, "command") || show_all;

    fprintf(f, "[");
    int first = 1;

    if (match_downloads) {
        if (!first) fprintf(f, ",");
        fprintf(f, "{\"name\":\"📁 Open Downloads\",\"path\":\"C:\\\\Users\\\\Public\\\\Downloads\",\"icon\":\"\",\"source\":\"QuickActions\"}");
        first = 0;
    }

    if (match_documents) {
        if (!first) fprintf(f, ",");
        fprintf(f, "{\"name\":\"📄 Open Documents\",\"path\":\"C:\\\\Users\\\\Public\\\\Documents\",\"icon\":\"\",\"source\":\"QuickActions\"}");
        first = 0;
    }

    if (match_desktop) {
        if (!first) fprintf(f, ",");
        fprintf(f, "{\"name\":\"🖥️ Open Desktop\",\"path\":\"C:\\\\Users\\\\Public\\\\Desktop\",\"icon\":\"\",\"source\":\"QuickActions\"}");
        first = 0;
    }

    if (match_temp) {
        if (!first) fprintf(f, ",");
        fprintf(f, "{\"name\":\"🗑️ Open Temp Folder\",\"path\":\"C:\\\\Windows\\\\Temp\",\"icon\":\"\",\"source\":\"QuickActions\"}");
        first = 0;
    }

    if (match_cmd) {
        if (!first) fprintf(f, ",");
        fprintf(f, "{\"name\":\"⌨️ Open Command Prompt\",\"path\":\"cmd.exe\",\"icon\":\"\",\"source\":\"QuickActions\"}");
        first = 0;
    }

    if (show_all) {
        // Add more default actions when no input
        if (!first) fprintf(f, ",");
        fprintf(f, "{\"name\":\"💻 Open System Settings\",\"path\":\"ms-settings:\",\"icon\":\"\",\"source\":\"QuickActions\"}");
    }

    fprintf(f, "]");
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
