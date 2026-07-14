const plainObject = (value) => value && typeof value === 'object' && !Array.isArray(value);

const validRuleProposal = (proposal) => plainObject(proposal)
  && typeof proposal.name === 'string'
  && proposal.name.trim()
  && Array.isArray(proposal.triggers)
  && proposal.triggers.length > 0
  && Array.isArray(proposal.actions)
  && proposal.actions.length > 0;

export const buildHabitRuleCreatePayload = (suggestion) => {
  if (!validRuleProposal(suggestion?.proposal)) {
    throw new Error('Invalid AI Brain rule proposal');
  }
  const sourceId = String(suggestion?.code || '').trim();
  if (!sourceId) {
    throw new Error('AI Brain rule source is missing');
  }
  return {
    ...suggestion.proposal,
    enable: true,
    source_type: 'ai_brain_suggestion',
    source_id: sourceId,
  };
};

const actionDescription = (action) => {
  if (action?.type === 'set_property') {
    const value = plainObject(action.value) || Array.isArray(action.value)
      ? JSON.stringify(action.value)
      : String(action.value);
    return `${action.deviceCode || '-'} / ${action.propertyKey || '-'} = ${value}`;
  }
  return action?.type || '-';
};

export const describeHabitRule = (suggestion) => {
  if (!validRuleProposal(suggestion?.proposal)) {
    throw new Error('Invalid AI Brain rule proposal');
  }
  return {
    name: suggestion.proposal.name,
    trigger: suggestion.proposal.triggers.map((trigger) => trigger.cronDesc || trigger.cronExpr || trigger.type || '-').join('; '),
    action: suggestion.proposal.actions.map(actionDescription).join('; '),
  };
};

export const isHabitRuleSuggestion = (suggestion) => suggestion?.type === 'automation_rule'
  && suggestion?.proposal_type === 'rule.create_draft'
  && validRuleProposal(suggestion.proposal);
