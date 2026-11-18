#ifndef LEVI_CLIENT_HEAP_GUARD_H
#define LEVI_CLIENT_HEAP_GUARD_H

typedef DWORD (WINAPI *SuspendThreadFn)(HANDLE);
typedef BOOLEAN (WINAPI *LockHeapFn)(PVOID);
static SuspendThreadFn original_suspend;
static LockHeapFn lock_heap, unlock_heap;

/* A frozen Wine thread must not own the heap needed by the hook installer.
 * Keep PreLoader's thread suspension and instruction-pointer handling intact;
 * acquire the process heap before each suspension, then release it immediately.
 */
static DWORD WINAPI suspend_without_heap_owner(HANDLE thread) {
    PVOID heap = GetProcessHeap();
    BOOLEAN locked = lock_heap(heap);
    DWORD result = original_suspend(thread);
    DWORD error = GetLastError();
    if (locked) unlock_heap(heap);
    SetLastError(error);
    return result;
}

static void install_heap_guard(void) {
    HMODULE ntdll = GetModuleHandleW(L"ntdll.dll");
    HMODULE preloader = GetModuleHandleW(L"PreLoader.dll");
    if (!preloader || !GetProcAddress(ntdll, "wine_get_version")) return;
    lock_heap = (LockHeapFn)(void *)GetProcAddress(ntdll, "RtlLockHeap");
    unlock_heap = (LockHeapFn)(void *)GetProcAddress(ntdll, "RtlUnlockHeap");
    if (!lock_heap || !unlock_heap) return;
    BYTE *base = (BYTE *)preloader;
    IMAGE_DOS_HEADER *dos = (IMAGE_DOS_HEADER *)base;
    if (dos->e_magic != IMAGE_DOS_SIGNATURE) return;
    IMAGE_NT_HEADERS64 *nt = (IMAGE_NT_HEADERS64 *)(base + dos->e_lfanew);
    if (nt->Signature != IMAGE_NT_SIGNATURE || nt->FileHeader.Machine != IMAGE_FILE_MACHINE_AMD64) return;
    DWORD rva = nt->OptionalHeader.DataDirectory[IMAGE_DIRECTORY_ENTRY_IMPORT].VirtualAddress;
    if (!rva) return;
    for (IMAGE_IMPORT_DESCRIPTOR *desc = (IMAGE_IMPORT_DESCRIPTOR *)(base + rva); desc->Name; ++desc) {
        if (!desc->OriginalFirstThunk) continue;
        IMAGE_THUNK_DATA64 *lookup = (IMAGE_THUNK_DATA64 *)(base + desc->OriginalFirstThunk);
        IMAGE_THUNK_DATA64 *iat = (IMAGE_THUNK_DATA64 *)(base + desc->FirstThunk);
        for (; lookup->u1.AddressOfData; ++lookup, ++iat) {
            if (IMAGE_SNAP_BY_ORDINAL64(lookup->u1.Ordinal)) continue;
            IMAGE_IMPORT_BY_NAME *name = (IMAGE_IMPORT_BY_NAME *)(base + lookup->u1.AddressOfData);
            if (strcmp((char *)name->Name, "SuspendThread")) continue;
            HMODULE pinned;
            if (!GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS | GET_MODULE_HANDLE_EX_FLAG_PIN,
                                   (LPCWSTR)&suspend_without_heap_owner, &pinned)) return;
            DWORD protection;
            if (!VirtualProtect(&iat->u1.Function, sizeof(void *), PAGE_READWRITE, &protection)) return;
            original_suspend = (SuspendThreadFn)(ULONG_PTR)iat->u1.Function;
            InterlockedExchangePointer((PVOID volatile *)&iat->u1.Function, (PVOID)&suspend_without_heap_owner);
            VirtualProtect(&iat->u1.Function, sizeof(void *), protection, &protection);
            OutputDebugStringA("LeviClient: Wine hook heap guard ready\n");
            return;
        }
    }
}
#endif
