void* alsactl_open();
void alsactl_close(void* handle);
int alsactl_get_volume(void* handle, char* name, long* min, long* max, long* val);
int alsactl_get_switch(void* handle, char* name, int* val);
int alsactl_get_enum(void* handle, char* name, char** result);

