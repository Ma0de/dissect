package webbuild

import "sort"

const stubFinder = `
import sys as _sys
from importlib.abc import MetaPathFinder as _MetaPathFinder, Loader as _Loader
from importlib.machinery import ModuleSpec as _ModuleSpec

class _StubLoader(_Loader):
    def create_module(self, spec):
        return None
    def exec_module(self, module):
        module.__getattr__ = lambda n: _Any()

class _StubFinder(_MetaPathFinder):
    def find_spec(self, fullname, path=None, target=None):
        if fullname == __name__ or fullname.startswith(__name__ + "."):
            return _ModuleSpec(fullname, _StubLoader())
        return None

_sys.meta_path.append(_StubFinder())
`

const ctypesStub = `class _Any:
    def __init__(self, *a, **k):
        pass
    def __call__(self, *a, **k):
        return 0
    def __getattr__(self, n):
        return _Any()
    def __getitem__(self, k):
        return _Any()
    def __setitem__(self, k, v):
        pass
    def __iter__(self):
        return iter(())

def __getattr__(name):
    return _Any()

class Structure:
    _fields_ = []
    def __init__(self, *a, **k):
        pass
    def __getattr__(self, n):
        return 0

class Union(Structure):
    pass

class Array(Structure):
    pass

def _class_factory(*a, **k):
    return type("_CT", (Structure,), {})

POINTER = _class_factory
WINFUNCTYPE = _class_factory
CFUNCTYPE = _class_factory

def byref(x, *a, **k):
    return _Any()

def sizeof(x):
    return 0

def cast(x, t):
    return x

def addressof(x):
    return 0

def create_string_buffer(x, *a, **k):
    return bytearray(x if isinstance(x, int) else len(x) + 1)

def string_at(x, *a, **k):
    return b""

c_int = int
c_uint = int
c_long = int
c_ulong = int
c_short = int
c_ushort = int
c_char = int
c_byte = int
c_ubyte = int
c_void_p = int
c_size_t = int
c_ssize_t = int
c_float = float
c_double = float
c_bool = bool
c_char_p = bytes
c_wchar_p = str
` + stubFinder

const wintypesStub = `HWND = int
HINSTANCE = int
HMODULE = int
DWORD = int
WORD = int
BYTE = int
UINT = int
LONG = int
BOOL = int
WPARAM = int
LPARAM = int
LPCWSTR = str
LPWSTR = str
LPSTR = bytes
LPVOID = int

class RECT:
    pass

class POINT:
    pass

class FILETIME:
    pass

class SIZE:
    pass
`

const subprocessStub = `PIPE = -1
STDOUT = -2
DEVNULL = -3
CREATE_NO_WINDOW = 0

class CompletedProcess:
    returncode = 1
    stdout = b""
    stderr = b""

class Popen:
    def __init__(self, *a, **k):
        self.returncode = 1
        self.pid = 0
    def poll(self):
        return 1
    def wait(self, *a, **k):
        return 1
    def communicate(self, *a, **k):
        return (b"", b"")
    def kill(self):
        pass
    def terminate(self):
        pass

def run(*a, **k):
    return CompletedProcess()

def call(*a, **k):
    return 1

def check_output(*a, **k):
    return b""

def check_call(*a, **k):
    return 1
`

const genericStub = `class _Any:
    def __init__(self, *a, **k):
        pass
    def __call__(self, *a, **k):
        return 0
    def __getattr__(self, n):
        return _Any()
    def __getitem__(self, k):
        return _Any()
    def __setitem__(self, k, v):
        pass
    def __iter__(self):
        return iter(())

def __getattr__(name):
    return _Any()
` + stubFinder

func stubEntries(mods []string) map[string][]byte {
	out := map[string][]byte{}
	for _, m := range mods {
		switch m {
		case "ctypes":
			out["ctypes/__init__.py"] = []byte(ctypesStub)
			out["ctypes/wintypes.py"] = []byte(wintypesStub)
		case "subprocess":
			out["subprocess.py"] = []byte(subprocessStub)
		default:
			out[m+".py"] = []byte(genericStub)
		}
	}
	return out
}

func sortedStubNames(stubs map[string][]byte) []string {
	names := make([]string, 0, len(stubs))
	for n := range stubs {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
