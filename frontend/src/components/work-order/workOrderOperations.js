export const operationsCopy = {
  policies: ['处置方案', 'Handling policies'], operations: ['运营队列', 'Operations'], refresh: ['刷新', 'Refresh'],
  blocked: ['待分派', 'Pending assignment'], claimable: ['待领取', 'Claimable'], breached: ['SLA 已超时', 'SLA breached'], failedDelivery: ['投递失败', 'Failed deliveries'],
  oldest: ['最早待分派时间', 'Oldest blocked assignment'], loading: ['加载中…', 'Loading…'], error: ['操作失败，请刷新后重试。', 'Operation failed. Refresh and retry.'], saved: ['已保存', 'Saved'],
  empty: ['暂无记录', 'No records'], reason: ['操作原因', 'Reason'], retry: ['重新投递', 'Retry delivery'], stop: ['停止投递', 'Stop delivery'],
  stopConfirm: ['确认停止投递此事件？', 'Stop delivery of this event?'], revokeConfirm: ['确认撤销此自动执行授权？', 'Revoke this automation grant?'],
  disableConfirm: ['确认禁用此处置方案？已建工单保留原快照。', 'Disable this handling policy? Existing orders retain their snapshots.'],
  attempts: ['已尝试次数', 'Attempts'], lastError: ['最近错误', 'Last error'], status: ['状态', 'Status'], pending: ['待处理', 'Pending'], dead: ['重试已耗尽', 'Attempts exhausted'], stopped: ['已停止', 'Stopped'],
  name: ['名称', 'Name'], code: ['编码', 'Code'], template: ['流程模板', 'Workflow template'], select: ['请选择', 'Select'], version: ['版本', 'Version'], newPolicy: ['新建方案', 'New policy'],
  publish: ['发布新版本', 'Publish new version'], edit: ['基于此版本编辑', 'Edit from this version'], enabled: ['已启用', 'Enabled'], disabled: ['已禁用', 'Disabled'], enable: ['启用', 'Enable'], disable: ['禁用', 'Disable'],
  node: ['处理环节', 'Task node'], location: ['责任区域', 'Responsibility area'], devices: ['设备编码（逗号分隔）', 'Device codes (comma separated)'], issue: ['问题类别编码', 'Issue category code'],
  role: ['责任角色', 'Responsible role'], broadcast: ['分派给全部成员', 'Assign to all members'], claim: ['候选人抢单', 'Candidate claiming'], delivery: ['分派方式', 'Delivery mode'],
  addRoute: ['添加责任规则', 'Add responsibility rule'], remove: ['移除', 'Remove'], removeConfirm: ['确认移除此规则？', 'Remove this rule?'],
  routeHint: ['空条件为默认规则，具体区域、设备及问题类别可覆盖默认规则。抢单要求该环节为或签。', 'Blank conditions define a default. Area, device and issue rules override it. Claiming requires an any-completion node.'],
  sla: ['处理时限', 'SLA'], useSLA: ['启用处理时限', 'Enable SLA'], responseMinutes: ['响应分钟数', 'Response minutes'], resolutionMinutes: ['解决分钟数', 'Resolution minutes'], warningMinutes: ['提前提醒分钟数', 'Warning minutes'],
  timezone: ['时区', 'Timezone'], weekdays: ['工作日', 'Working days'], calendarHint: ['不选工作日表示全天候；假期按所选时区计算。', 'No selected weekdays means 24/7. Holidays use the selected timezone.'], startHour: ['开始小时', 'Start hour'], endHour: ['结束小时', 'End hour'],
  holidays: ['假期日期（逗号分隔）', 'Holiday dates (comma separated)'], pauseReasons: ['允许的暂停原因（逗号分隔）', 'Allowed pause reasons (comma separated)'],
  grants: ['自动执行授权', 'Automation grants'], expiry: ['到期时间', 'Expires at'], createGrant: ['创建授权', 'Create grant'], revoke: ['撤销授权', 'Revoke grant'], revoked: ['已撤销', 'Revoked'], expired: ['已过期', 'Expired'],
  preview: ['预览责任人', 'Preview responsibility'], users: ['责任成员', 'Members'], no_active_members: ['没有可用成员，将进入待分派队列', 'No active members; the task will wait for assignment'], role_unavailable: ['角色不可用，将进入待分派队列', 'Role unavailable; the task will wait for assignment'],
  claimTask: ['领取任务', 'Claim task'], responseDue: ['响应截止', 'Response deadline'], resolutionDue: ['解决截止', 'Resolution deadline'], responded: ['已响应', 'Responded'], completed: ['已结束', 'Completed'], paused: ['已暂停', 'Paused'], running: ['计时中', 'Running'],
  pause: ['暂停计时', 'Pause SLA'], resume: ['恢复计时', 'Resume SLA'], pinned: ['固定方案版本', 'Pinned policy version'], device: ['设备编码', 'Device code'],
  monday: ['周一', 'Mon'], tuesday: ['周二', 'Tue'], wednesday: ['周三', 'Wed'], thursday: ['周四', 'Thu'], friday: ['周五', 'Fri'], saturday: ['周六', 'Sat'], sunday: ['周日', 'Sun']
}
export function operationText(locale, key) { return operationsCopy[key]?.[String(locale).startsWith('en') ? 1 : 0] ?? key }
export function candidateTasks(tasks, actorID, cycle) { return (tasks || []).filter(task => task.workflow_cycle === cycle && task.status === 'pending' && task.delivery_mode === 'claim' && !task.assignee_user_id && task.candidate_user_ids?.includes(Number(actorID))) }
export function commaValues(value) { return [...new Set(String(value || '').split(/[,，\n]/).map(v => v.trim()).filter(Boolean))] }
export const operationID = value => value?.ID || value?.id || 0
