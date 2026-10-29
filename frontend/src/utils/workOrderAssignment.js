export function setWorkOrderAssignmentMode(node, mode) {
  if (mode === 'project_role') {
    node.assignment = { kind: 'project_role', role_id: 0 }
    node.participant_user_ids = []
  } else {
    delete node.assignment
  }
}

export function workOrderAssignmentCopy(locale) {
  return String(locale).startsWith('zh') ? {
    title: '待分派', hint: '当前节点没有可用处理人。已完成的工作已保留，补充分派后继续流转。',
    no_active_members: '该角色暂无可用成员', member_unavailable: '指定成员已停用或已移出项目', role_unavailable: '所选角色已删除或不再适用于此项目',
    unavailable: '当前分派规则无法找到可用成员', retry: '重新解析成员', member: '人工指定处理人', choose: '请选择项目成员',
    reason: '人工分派原因（必填）', assign: '确认人工分派', refresh: '刷新', loading: '正在加载分派状态…',
    failed: '分派操作未成功，请刷新工单后重试。', loadFailed: '无法加载分派状态，请重试。',
  } : {
    title: 'Pending assignment', hint: 'This node has no available assignee. Completed work is saved; assign a member to continue.',
    no_active_members: 'This role has no active members', member_unavailable: 'A selected member is inactive or has left this project', role_unavailable: 'The selected role was deleted or is no longer available in this project',
    unavailable: 'The assignment rule has no available members', retry: 'Resolve members again', member: 'Assign a project member', choose: 'Select a project member',
    reason: 'Reason for manual assignment (required)', assign: 'Confirm manual assignment', refresh: 'Refresh', loading: 'Loading assignment status…',
    failed: 'Assignment failed. Refresh the work order and try again.', loadFailed: 'Unable to load assignment status. Please retry.',
  }
}
