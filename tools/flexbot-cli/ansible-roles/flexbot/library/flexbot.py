# Copyright: (c) 2020, Igor Feoktistov <ifeoktistov@yahoo.com>
# The MIT License (MIT)

from __future__ import absolute_import, division, print_function
__metaclass__ = type

ANSIBLE_METADATA = {'metadata_version': '1.1',
                    'status': ['preview'],
                    'supported_by': 'community'}

import sys
import os
import json
import subprocess

from ansible.module_utils.basic import AnsibleModule

def main():
    config = None
    global module
    module = AnsibleModule(
        argument_spec=dict(
            flexbot=dict(required=True, type='str'),
            config=dict(required=True, type='str'),
            op=dict(required=True, type='str'),
            host=dict(required=True, type='str'),
            image=dict(required=False, type='str'),
            template=dict(required=False, type='str'),
            snapshot=dict(required=False, type='str')
        )
    )
    config = json.loads(module.params['config'])
    if config is not None:
        if 'op' not in module.params:
            module.fail_json(msg="expected \"op\" parameter")
        if 'host' in module.params:
            config['compute']['hostName'] = module.params['host']
        else:
            module.fail_json(msg="expected \"host\" parameter")
        if 'dataLun' in config['storage'] and 'size' in config['storage']['dataLun']:
            config['storage']['dataLun']['size'] = int(config['storage']['dataLun']['size'])
        if 'bootLun' in config['storage'] and 'size' in config['storage']['bootLun']:
            config['storage']['bootLun']['size'] = int(config['storage']['bootLun']['size'])
        if module.params['op'] == 'provisionServer':
            if 'image' in module.params:
                config['storage']['bootLun']['osImage'] = {'name':  module.params['image']}
            else:
                module.fail_json(msg="expected \"image\" parameter")
            if 'template' in module.params:
                config['storage']['seedLun'] = {'seedTemplate':{'location': module.params['template']}}
            else:
                module.fail_json(msg="expected \"template\" parameter")
        if not(module.params['op'] == 'provisionServer' or
                module.params['op'] == 'deprovisionServer' or
                module.params['op'] == 'startServer' or
                module.params['op'] == 'stopServer' or
                module.params['op'] == 'createSnapshot' or
                module.params['op'] == 'deleteSnapshot' or
                module.params['op'] == 'restoreSnapshot'):
                module.fail_json(msg="unexpected \"op\" parameter: %s" % module.params['op'])
        flexbot = module.params['flexbot']
        if flexbot is None:
            module.fail_json(msg="expected \"flexbot\" parameter")
        try:
            process_args = [flexbot,"--op=" + module.params['op'],"--encodingFormat=json"]
            if (module.params['op'] == 'createSnapshot' or
                module.params['op'] == 'deleteSnapshot' or
                module.params['op'] == 'restoreSnapshot'):
                if 'snapshot' in module.params:
                    process_args.append("--snapshot=" + module.params['snapshot'])
                else:
                    module.fail_json(msg="expected \"snapshot\" parameter")
            if sys.version_info[0] >= 3:
                p = subprocess.Popen(process_args,
                        stdin=subprocess.PIPE,
                        stdout=subprocess.PIPE,
                        stderr=subprocess.PIPE,
                        text=True)
            else:
                p = subprocess.Popen(process_args,
                        stdin=subprocess.PIPE,
                        stdout=subprocess.PIPE,
                        stderr=subprocess.PIPE)
        except Exception as e:
            module.fail_json(msg="Popen(flexbot=%s): %s" % (flexbot, e))
        output, error = p.communicate(input=json.dumps(config))
        if p.returncode != 0:
            module.fail_json(msg="Popen(flexbot=%s): %s %s" % (flexbot, output, error))
        try:
            response = json.loads(output)
        except Exception as e:
            module.fail_json(msg="incorrect argument: %s" % output)
        if response['status'] == "failure":
            module.fail_json(msg="flexbot failure: %s" % response['errorMessage'])
        if (module.params['op'] == 'provisionServer' or
            module.params['op'] == 'deprovisionServer' or
            module.params['op'] == 'startServer' or
            module.params['op'] == 'stopServer'):
            module.exit_json(changed=True, response=json.dumps(response['server']))
        else:
            module.exit_json(changed=True, response=response['status'])
    else:
        module.fail_json(msg="Failure to load configuration JSON %s" % module.params['config'])


if __name__ == '__main__':
    main()
