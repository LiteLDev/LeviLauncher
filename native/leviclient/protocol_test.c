#include <assert.h>
#include <string.h>
#include "protocol.h"
int main(void) {
    char code[9];
    assert(levi_device_code(L"Open the page and enter the code\n\nAB12CD34\n\nContinue", code));
    assert(!strcmp(code, "AB12CD34"));
    assert(!levi_device_code(L"An unrelated warning", code));
    assert(!levi_device_code(L"enter the code\nAB12CD345", code));
    assert(!levi_device_code(L"enter the code\nAB12\n", code));
    assert(!levi_device_code(L"enter the code\nAB12&X34", code));
    return 0;
}
