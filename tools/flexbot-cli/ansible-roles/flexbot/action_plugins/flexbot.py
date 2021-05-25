# Copyright: (c) 2020, Igor Feoktistov <ifeoktistov@yahoo.com>
# The MIT License (MIT)

from __future__ import absolute_import, division, print_function
__metaclass__ = type

import os
import platform

from ansible.errors import AnsibleError
from ansible.plugins.action import ActionBase


class ActionModule(ActionBase):
    def run(self, tmp=None, task_vars=None):
        super(ActionModule, self).run(tmp, task_vars)
        module_args = self._task.args.copy()
        if self._task._role is not None:
            module_args['flexbot'] = self._task._role._role_path + "/bin/flexbot." + platform.system().lower()
        else:
            raise AnsibleError("Expected flexbot role in task")
        module_return = self._execute_module(module_name='flexbot',
                                            module_args=module_args,
                                            task_vars=task_vars, tmp=tmp)
        return module_return
