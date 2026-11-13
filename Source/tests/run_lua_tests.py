import ctypes, os
root=str(__import__("pathlib").Path(__file__).resolve().parents[2]);os.chdir(root)
lib=ctypes.CDLL('liblua5.4.so.0');lib.luaL_newstate.restype=ctypes.c_void_p;lib.luaL_openlibs.argtypes=[ctypes.c_void_p];lib.luaL_loadfilex.argtypes=[ctypes.c_void_p,ctypes.c_char_p,ctypes.c_char_p];lib.luaL_loadfilex.restype=ctypes.c_int;lib.lua_pcallk.argtypes=[ctypes.c_void_p,ctypes.c_int,ctypes.c_int,ctypes.c_int,ctypes.c_ssize_t,ctypes.c_void_p];lib.lua_pcallk.restype=ctypes.c_int;lib.lua_tolstring.argtypes=[ctypes.c_void_p,ctypes.c_int,ctypes.c_void_p];lib.lua_tolstring.restype=ctypes.c_char_p;lib.lua_close.argtypes=[ctypes.c_void_p];lib.luaL_openlibs.argtypes=[ctypes.c_void_p]
for f in ['Source/tests/touchpad_first_binding_contract.lua','Source/tests/touchpad_mouse_policy_contract.lua','Source/tests/mic_mute_vehicle_sync.lua','Source/tests/mic_mute_lifecycle_contract.lua','Source/tests/r2_sliders_packet_contract.lua','Source/tests/motion_steering_endstops.lua','Source/tests/mod_lifecycle_contract.lua']:
 L=lib.luaL_newstate();lib.luaL_openlibs(L)
 x=lib.luaL_loadfilex(L,f.encode(),None)
 if x:raise RuntimeError('compile '+f+': '+str(lib.lua_tolstring(L,-1,None)))
 x=lib.lua_pcallk(L,0,0,0,0,None)
 if x:raise RuntimeError('execute '+f+': '+str(lib.lua_tolstring(L,-1,None)))
 lib.lua_close(L);print('PASS',f)
