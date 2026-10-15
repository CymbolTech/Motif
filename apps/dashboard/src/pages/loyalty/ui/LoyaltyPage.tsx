import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';
import { IconAlertTriangle, IconArrowDownRight, IconArrowUpRight, IconChevronDown, IconCircleCheck, IconCircleDashed, IconCoin, IconGift, IconRefresh, IconSearch, IconSparkles } from '@tabler/icons-react';
import { createAdminPointTransaction, fetchAdminRedemptions, fetchAdminUserProfile, fetchAdminUserTransactions, fetchRedemptionAttempts, fetchRewards, redeemAdminReward, retryAdminRedemption, type AdminRedemption, type PointBalance, type PointTransaction, type Redemption, type RedemptionAttempt, type Reward, type UserAchievement, type UserMission, type UserProgression } from '../../../shared/api/client';
import { useScope } from '../../../shared/scope/ScopeContext';
import { CustomSelect } from '../../../shared/ui/CustomSelect';
import { GuidePanel } from '../../../shared/ui/GuidePanel';
import { PageHeader } from '../../../shared/ui/PageHeader';
import { formatDateTime } from '../../../shared/lib/format';

type LoyaltyData = {
  balances: PointBalance[];
  transactions: PointTransaction[];
  missions: UserMission[];
  achievements: UserAchievement[];
  progression: UserProgression;
};

export function LoyaltyPage({ onNavigate }: { onNavigate: (view: 'currency' | 'mission' | 'achievement' | 'reward') => void }) {
  const { scope, loading: scopeLoading, error: scopeError } = useScope();
  const [userInput, setUserInput] = useState('user-42');
  const [userId, setUserId] = useState('user-42');
  const [data, setData] = useState<LoyaltyData>({ balances: [], transactions: [], missions: [], achievements: [], progression: { user_id: 'user-42', total_xp: 0, level: 0, level_name: 'Unranked' } });
  const [rewards, setRewards] = useState<Reward[]>([]);
  const [redemptions, setRedemptions] = useState<AdminRedemption[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [redeeming, setRedeeming] = useState<string | null>(null);
  const [retrying, setRetrying] = useState<string | null>(null);
  const [lastRedemption, setLastRedemption] = useState<Redemption | null>(null);
  const [ledgerCursor, setLedgerCursor] = useState<string | undefined>();
  const [ledgerLoadingMore, setLedgerLoadingMore] = useState(false);
  const [attemptsByRedemption, setAttemptsByRedemption] = useState<Record<string, RedemptionAttempt[]>>({});
  const [openAttempts, setOpenAttempts] = useState<string | null>(null);
  const [attemptsLoading, setAttemptsLoading] = useState<string | null>(null);
  const [ledgerForm, setLedgerForm] = useState({ type: 'award' as 'award' | 'spend', currency: '', amount: '10' });
  const [ledgerSaving, setLedgerSaving] = useState(false);

  const refresh = useCallback(async (targetUser = userId) => {
    setLoading(true);
    setError(null);
    try {
      if (!scope) {
        setError(scopeError ?? 'Your workspace membership is not available yet.');
        return;
      }
      const [profile, catalog, recentRedemptions] = await Promise.all([
        fetchAdminUserProfile(scope, targetUser),
        fetchRewards(scope),
        fetchAdminRedemptions(8, { tenant_id: scope.tenant_id, environment_id: scope.environment_id }),
      ]);
      setData({ balances: profile.balances, transactions: profile.transactions, missions: profile.missions, achievements: profile.achievements, progression: profile.progression });
      setLedgerCursor(profile.next_cursor);
      setRewards(catalog.filter((reward) => reward.active));
      setRedemptions(recentRedemptions);
    } catch (refreshError) {
      setError(refreshError instanceof Error ? refreshError.message : 'Loyalty data could not be loaded.');
    } finally {
      setLoading(false);
    }
  }, [scope, scopeError, userId]);

  useEffect(() => { void refresh(); }, [refresh]);
  useEffect(() => { setLastRedemption(null); }, [scope?.tenant_id, scope?.environment_id]);

  function handleUserSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextUser = userInput.trim();
    if (!nextUser) return;
    setUserId(nextUser);
  }

  async function handleRedeem(reward: Reward) {
    setRedeeming(reward.id);
    setError(null);
    try {
      if (!scope) throw new Error('Your workspace membership is still loading.');
      const redemption = await redeemAdminReward(scope, reward.id, {
        external_user_id: userId,
        idempotency_key: `console-${reward.id}-${userId}`,
      });
      setLastRedemption(redemption);
      await refresh(userId);
    } catch (redeemError) {
      setError(redeemError instanceof Error ? redeemError.message : 'Reward could not be redeemed.');
    } finally {
      setRedeeming(null);
    }
  }

  async function handleRetry(redemption: AdminRedemption) {
    setRetrying(redemption.id);
    setError(null);
    try {
      if (!scope) throw new Error('Your workspace membership is still loading.');
      const retried = await retryAdminRedemption(scope, redemption.id);
      setLastRedemption(retried);
      await refresh(userId);
    } catch (retryError) {
      setError(retryError instanceof Error ? retryError.message : 'Redemption could not be retried.');
    } finally {
      setRetrying(null);
    }
  }

  async function loadMoreLedger() {
    if (!ledgerCursor) return;
    setLedgerLoadingMore(true);
    try {
      if (!scope) throw new Error('Your workspace membership is still loading.');
      const next = await fetchAdminUserTransactions(scope, userId, 12, ledgerCursor);
      setData((current) => ({ ...current, transactions: [...current.transactions, ...next.transactions] }));
      setLedgerCursor(next.next_cursor);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'More ledger entries could not be loaded.');
    } finally {
      setLedgerLoadingMore(false);
    }
  }

  async function toggleAttempts(redemption: AdminRedemption) {
    if (!scope) return;
    if (openAttempts === redemption.id) { setOpenAttempts(null); return; }
    setOpenAttempts(redemption.id);
    if (attemptsByRedemption[redemption.id]) return;
    setAttemptsLoading(redemption.id);
    try {
      const attempts = await fetchRedemptionAttempts(redemption.id, scope);
      setAttemptsByRedemption((current) => ({ ...current, [redemption.id]: attempts }));
    } catch (attemptError) {
      setError(attemptError instanceof Error ? attemptError.message : 'Redemption attempts could not be loaded.');
    } finally { setAttemptsLoading(null); }
  }

  async function submitLedger(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!scope) return;
    const amount = Number(ledgerForm.amount);
    if (!ledgerForm.currency || !Number.isInteger(amount) || amount <= 0) { setError('Choose a currency and a positive whole number.'); return; }
    setLedgerSaving(true);
    setError(null);
    try {
      await createAdminPointTransaction({ tenant_id: scope.tenant_id, environment_id: scope.environment_id, external_user_id: userId, currency: ledgerForm.currency, amount, type: ledgerForm.type, idempotency_key: `console-${ledgerForm.type}-${userId}-${ledgerForm.currency}-${Date.now()}`, provenance: { source: 'admin_console' } });
      await refresh(userId);
    } catch (ledgerError) {
      setError(ledgerError instanceof Error ? ledgerError.message : 'Point transaction could not be recorded.');
    } finally { setLedgerSaving(false); }
  }

  const totalPoints = useMemo(() => data.balances.filter((balance) => balance.currency !== 'xp').reduce((total, balance) => total + balance.balance, 0), [data.balances]);
  const balanceFor = (currency?: string) => data.balances.find((balance) => balance.currency === currency)?.balance ?? 0;

  const formatDate = (value: string) => formatDateTime(value);
  const percent = (value: number, total: number) => total > 0 ? Math.min(100, Math.round((value / total) * 100)) : 0;
  const redemptionTone = (status: string) => status === 'fulfilled' ? 'success' : status === 'failed' ? 'danger' : 'warning';

  return (
    <main className="page page-loyalty">
      <PageHeader breadcrumb={['Workspace', 'Points & rewards']} title="Points & rewards" description="Review a member's loyalty account, adjust their points, and redeem rewards in this workspace." topbarActions={<button className="icon-button ghost compact" type="button" onClick={() => void refresh()} aria-label="Refresh points and rewards"><IconRefresh size={15} stroke={1.75} className={loading ? 'spin' : ''} /></button>} />

      <GuidePanel id="guide-loyalty" title="Points & rewards" purpose="Inspect one user's balances and progression, make a manual ledger change, or redeem a reward." steps={['Enter the external user ID you want to inspect.', 'Review balances, XP, missions, and achievements.', 'Award or spend points, then redeem an eligible reward.']} example="Load user-42, award 10 coins, then redeem a 500-coin voucher." result="The ledger records the change and the redemption moves through provider delivery." />

      <section id="loyalty-user" className="panel member-bar" aria-label="Choose a member">
        <form className="member-lookup" onSubmit={handleUserSubmit}>
          <label className="search-field"><IconSearch size={15} stroke={1.75} aria-hidden="true" /><input className="mono" value={userInput} onChange={(event) => setUserInput(event.target.value)} placeholder="External user ID, e.g. user-42" aria-label="External user ID" spellCheck={false} /></label>
          <button className="primary-button" type="submit" disabled={!userInput.trim()}>Load account</button>
        </form>
        <p className="member-bar-note">Showing <span className="mono">{userId}</span>. Everything below is scoped to this member{scope ? ` in ${scope.environment_name}` : ''}.</p>
      </section>

      {error && <p className="alert alert-error" role="alert"><IconAlertTriangle size={16} aria-hidden="true" /><span><strong>Points &amp; rewards could not be loaded.</strong> {error}</span></p>}
      {!error && !scope && scopeLoading && <p className="alert">Connecting to your workspace…</p>}
      {lastRedemption && <div className="receipt" aria-live="polite"><span className={`badge capitalize badge-${redemptionTone(lastRedemption.status)}`}><span className="dot" />{lastRedemption.status}</span><span className="receipt-copy"><strong>{lastRedemption.status === 'fulfilled' ? 'Reward fulfilled' : `Redemption ${lastRedemption.status}`}</strong><code>{lastRedemption.provider_reference ?? lastRedemption.id}</code></span><span className="receipt-note">Provider delivery</span></div>}

      <section className="metrics metrics-auto" aria-label="Point summary">
        <div className="metric"><span className="metric-label">Available balance</span><strong className="metric-value">{loading ? '–' : totalPoints.toLocaleString()}</strong><span className="metric-note">Spendable points across currencies</span></div>
        {data.balances.slice(0, 2).map((balance) => <div className="metric" key={balance.currency_id}><span className="metric-label">{balance.name}</span><strong className="metric-value">{loading ? '–' : balance.balance.toLocaleString()}</strong><span className="metric-note mono">{balance.currency}</span></div>)}
        {data.balances.length === 0 && !loading && <div className="metric"><span className="metric-label">No balances yet</span><span className="metric-note metric-note-wrap">The first award will create an account.</span><button className="text-button accent metric-action" type="button" onClick={() => onNavigate('currency')}>Manage currencies</button></div>}
        <div className="metric"><span className="metric-label">Progression</span><strong className="metric-value">{loading ? '–' : `Level ${data.progression.level}`}</strong><span className="metric-note">{loading ? 'Loading XP' : `${data.progression.total_xp.toLocaleString()} XP · ${data.progression.level_name}`}</span></div>
      </section>

      <div className="loyalty-grid">
        <div className="loyalty-col">
          <section id="loyalty-activity" className="panel" aria-labelledby="ledger-title">
            <div className="panel-header"><div><h2 id="ledger-title">Point activity</h2><p>The newest awards, spends, and redemptions for this user.</p></div><span className="panel-meta">{data.transactions.length} {data.transactions.length === 1 ? 'entry' : 'entries'}</span></div>
            {loading ? <div className="skeleton-rows"><span /><span /><span /></div> : data.transactions.length === 0 ? <div className="empty-state"><IconCoin size={20} stroke={1.5} aria-hidden="true" /><strong>No transactions yet</strong><span>Point awards and redemptions will appear here for this user.</span></div> : <><ul className="ledger-list">{data.transactions.map((transaction) => <li key={transaction.id}><span className={`ledger-mark ${transaction.amount < 0 ? 'is-spend' : 'is-award'}`} aria-hidden="true">{transaction.amount < 0 ? <IconArrowDownRight size={14} stroke={1.75} /> : <IconArrowUpRight size={14} stroke={1.75} />}</span><span className="ledger-copy"><strong>{transaction.entry_type.replaceAll('_', ' ')}</strong><small><span className="mono">{transaction.currency_code}</span> · {formatDate(transaction.created_at)}</small></span><span className={`ledger-amount ${transaction.amount < 0 ? 'is-negative' : 'is-positive'}`}>{transaction.amount > 0 ? '+' : ''}{transaction.amount.toLocaleString()}</span></li>)}</ul>{ledgerCursor && <div className="panel-footer"><button className="secondary-button compact" type="button" onClick={() => void loadMoreLedger()} disabled={ledgerLoadingMore}>{ledgerLoadingMore ? 'Loading…' : 'Load more ledger entries'}</button></div>}</>}
          </section>

          <section className="panel" aria-labelledby="progression-title">
            <div className="panel-header"><div><h2 id="progression-title">Progression</h2><p>Missions, achievements, and XP level recorded for this user.</p></div><span className="panel-meta">{data.missions.length + data.achievements.length} {data.missions.length + data.achievements.length === 1 ? 'track' : 'tracks'}</span></div>
            {loading ? <div className="skeleton-rows"><span /><span /></div> : <>
              <div className="xp-strip"><div><strong>{data.progression.level_name} · Level {data.progression.level}</strong><span>{data.progression.total_xp.toLocaleString()} XP{data.progression.next_level_xp ? ` · ${Math.max(0, data.progression.next_level_xp - data.progression.total_xp).toLocaleString()} to next level` : ' · top level reached'}</span></div>{data.progression.next_level_xp ? <div className="meter" role="meter" aria-label="Progress to next level" aria-valuemin={0} aria-valuemax={data.progression.next_level_xp} aria-valuenow={data.progression.total_xp}><i style={{ width: percent(data.progression.total_xp, data.progression.next_level_xp) + '%' }} /></div> : null}</div>
              {data.missions.length === 0 && data.achievements.length === 0 ? <div className="empty-state"><IconSparkles size={20} stroke={1.5} aria-hidden="true" /><strong>No progression tracks yet</strong><span>Add missions or achievements to track progress for members.</span><div className="empty-state-actions"><button className="secondary-button compact" type="button" onClick={() => onNavigate('mission')}>Open missions</button><button className="secondary-button compact" type="button" onClick={() => onNavigate('achievement')}>Open achievements</button></div></div> : <div className="progression-columns">
                <div><h3 className="column-heading">Missions</h3>{data.missions.length === 0 ? <p className="column-empty">No missions configured.</p> : <ul className="track-list">{data.missions.map((mission) => <li key={mission.id}><span className={`track-mark ${mission.completed ? 'is-complete' : ''}`} aria-hidden="true">{mission.completed ? <IconCircleCheck size={15} stroke={1.75} /> : <IconCircleDashed size={15} stroke={1.75} />}</span><span className="track-copy"><strong>{mission.name}</strong><small>{mission.progress} / {mission.target_count} · <span className="mono">{mission.event_type}</span></small><span className="meter"><i style={{ width: percent(mission.progress, mission.target_count) + '%' }} /></span></span><span className="track-status">{mission.completed ? 'Complete' : `${percent(mission.progress, mission.target_count)}%`}</span></li>)}</ul>}</div>
                <div><h3 className="column-heading">Achievements</h3>{data.achievements.length === 0 ? <p className="column-empty">No achievements configured.</p> : <ul className="track-list">{data.achievements.map((achievement) => <li key={achievement.id}><span className={`track-mark ${achievement.unlocked ? 'is-complete' : ''}`} aria-hidden="true">{achievement.unlocked ? <IconCircleCheck size={15} stroke={1.75} /> : <IconCircleDashed size={15} stroke={1.75} />}</span><span className="track-copy"><strong>{achievement.name}</strong><small>{achievement.progress} / {achievement.required_count} · <span className="mono">{achievement.event_type}</span></small><span className="meter"><i style={{ width: percent(achievement.progress, achievement.required_count) + '%' }} /></span></span><span className="track-status">{achievement.unlocked ? 'Unlocked' : `${percent(achievement.progress, achievement.required_count)}%`}</span></li>)}</ul>}</div>
              </div>}
            </>}
          </section>

          <section className="panel" aria-labelledby="redemptions-title">
            <div className="panel-header"><div><h2 id="redemptions-title">Reward redemptions</h2><p>Track provider delivery, inspect attempts, and retry failed redemptions.</p></div><span className="panel-meta">{redemptions.length} {redemptions.length === 1 ? 'record' : 'records'}</span></div>
            {loading ? <div className="skeleton-rows"><span /><span /></div> : redemptions.length === 0 ? <div className="empty-state"><IconGift size={20} stroke={1.5} aria-hidden="true" /><strong>No redemptions yet</strong><span>Successful and failed reward deliveries will appear here.</span></div> : <ul className="redemption-list">{redemptions.map((redemption) => <li key={redemption.id}><div className="redemption-row"><span className={`badge capitalize badge-${redemptionTone(redemption.status)}`}><span className="dot" />{redemption.status}</span><span className="redemption-copy"><strong>{redemption.reward_name}</strong><small><span className="mono">{redemption.external_user_id}</span> · {formatDate(redemption.created_at)} · {redemption.attempt_count ?? 0} attempts</small>{redemption.status === 'delivering' && <small>{redemption.reconciliation_attempts ?? 0} reconciliation checks{redemption.last_provider_check_error ? ` · ${redemption.last_provider_check_error}` : ''}</small>}</span><code className="redemption-reference">{redemption.provider_reference ?? redemption.id}</code><span className="redemption-actions">{redemption.status === 'failed' && redemption.external_user_id === userId && <button className="text-button accent" type="button" onClick={() => void handleRetry(redemption)} disabled={retrying === redemption.id}>{retrying === redemption.id ? 'Retrying…' : 'Retry'}</button>}<button className="text-button" type="button" aria-expanded={openAttempts === redemption.id} onClick={() => void toggleAttempts(redemption)}>Attempts<IconChevronDown size={14} stroke={1.75} className={openAttempts === redemption.id ? 'is-flipped' : ''} /></button></span></div>{openAttempts === redemption.id && <div className="attempt-panel">{attemptsLoading === redemption.id ? <span className="column-empty">Loading attempts…</span> : (attemptsByRedemption[redemption.id] ?? []).length === 0 ? <span className="column-empty">No delivery attempts recorded.</span> : (attemptsByRedemption[redemption.id] ?? []).map((attempt) => <div className="attempt-row" key={attempt.id}><span className="badge badge-outline capitalize">{attempt.kind}</span><span><strong>{attempt.status}</strong><small>{attempt.provider_state || attempt.error_message || 'No provider detail'} · {formatDate(attempt.created_at)}</small></span></div>)}</div>}</li>)}</ul>}
          </section>
        </div>

        <div className="loyalty-col">
          <section id="loyalty-actions" className="panel" aria-labelledby="ledger-controls-title">
            <div className="panel-header"><div><h2 id="ledger-controls-title">Change point balance</h2><p>Each change is added to this user's append-only ledger.</p></div></div>
            <form className="panel-body form-stack" onSubmit={(event) => void submitLedger(event)}>
              <div className="field"><span className="field-label">Operation</span><div className="segmented" role="group" aria-label="Operation"><button type="button" aria-pressed={ledgerForm.type === 'award'} onClick={() => setLedgerForm({ ...ledgerForm, type: 'award' })}>Award</button><button type="button" aria-pressed={ledgerForm.type === 'spend'} onClick={() => setLedgerForm({ ...ledgerForm, type: 'spend' })}>Spend</button></div></div>
              <div className="field"><span className="field-label">Currency</span><CustomSelect value={ledgerForm.currency} onChange={(value) => setLedgerForm({ ...ledgerForm, currency: value })} ariaLabel="Currency" placeholder="Select currency" options={data.balances.map((balance) => ({ value: balance.currency, label: `${balance.name} · ${balance.currency}` }))} />{data.balances.length === 0 && !loading && <small className="field-help">This user has no balances yet. Award points through an automation first.</small>}</div>
              <label className="field"><span className="field-label">Amount</span><input required type="number" min="1" step="1" value={ledgerForm.amount} onChange={(event) => setLedgerForm({ ...ledgerForm, amount: event.target.value })} /></label>
              <button className="primary-button full" type="submit" disabled={ledgerSaving || !scope}>{ledgerSaving ? 'Recording…' : `Record ${ledgerForm.type}`}</button>
            </form>
          </section>

          <section className="panel" aria-labelledby="rewards-title">
            <div className="panel-header"><div><h2 id="rewards-title">Available rewards</h2><p>Compare each cost with the balance, then redeem for this user.</p></div></div>
            {loading ? <div className="skeleton-rows"><span /><span /></div> : rewards.length === 0 ? <div className="empty-state"><IconGift size={20} stroke={1.5} aria-hidden="true" /><strong>No rewards configured</strong><span>Create a reward to make it available here.</span><button className="secondary-button compact" type="button" onClick={() => onNavigate('reward')}>Open rewards</button></div> : <ul className="reward-list">{rewards.map((reward) => { const balance = balanceFor(reward.cost_currency); const cost = reward.cost_amount ?? 0; const canRedeem = !cost || balance >= cost; return <li key={reward.id}><span className="reward-copy"><strong>{reward.name}</strong><small>{reward.description || 'Digital unlock'}</small></span><span className="reward-cost">{cost ? <>{cost.toLocaleString()} <span className="mono">{reward.cost_currency}</span></> : 'Free'}</span><button className={canRedeem ? 'secondary-button compact' : 'text-button compact'} type="button" disabled={!canRedeem || redeeming === reward.id} onClick={() => void handleRedeem(reward)}>{redeeming === reward.id ? 'Delivering…' : canRedeem ? 'Redeem' : 'Need points'}</button></li>; })}</ul>}
          </section>
        </div>
      </div>
    </main>
  );
}
