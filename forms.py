from django import forms
from dcim.models import Device
from utilities.forms.fields import DynamicModelMultipleChoiceField

class DeviceImpactAnalysisForm(forms.Form):
    devices = DynamicModelMultipleChoiceField(
        queryset=Device.objects.all(),
        label='Select Devices',
        help_text='Select one or more devices to analyze for circuit impact.',
        required=True
    )