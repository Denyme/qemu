sudo pacman -S openvswitch
sudo systemctl enable --now openvswitch
sudo systemctl start openvswitch
sudo ovs-vsctl add-br br-qemu
sudo virsh net-destroy default
sudo virsh net-autostart --disable default
sudo virsh net-define ovs-network.xml
sudo virsh net-start ovs-network
sudo virsh net-autostart ovs-network
