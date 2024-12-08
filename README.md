# rpi-manager

A simple, lightweight, and easy-to-use web-based Raspberry Pi management tool.
```sh
docker build -t rpi-manager .

# Run with systemd support (recommended for full functionality)
docker run --privileged -v /bin/systemctl:/bin/systemctl -v /run/systemd/system:/run/systemd/system -v /var/run/dbus/system_bus_socket:/var/run/dbus/system_bus_socket -v /sys/fs/cgroup:/sys/fs/cgroup -v /usr/sbin:/usr/sbin -p 8080:8080 --name rpi-manager rpi-manager

# Run without systemd support (limited functionality)
#docker run --privileged -p 8080:8080 --name rpi-manager rpi-manager
```

