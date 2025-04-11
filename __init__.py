from extras.plugins import PluginConfig

class NetBoxImactConfig(PluginConfig):
    name = 'netbox_imact'
    verbose_name = 'NetBox Impact Analysis'
    description = 'Analyze the impact of device outages on circuits'
    version = '0.1.0'
    author = 'Lorenzo Karel'
    author_email = 'r2unit@proton.me'
    base_url = 'impact-analysis'
    required_settings = []
    default_settings = {}

config = NetBoxImactConfig