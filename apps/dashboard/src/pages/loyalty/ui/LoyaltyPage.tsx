import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';
import { IconArrowDownRight, IconCircleCheck, IconCoin, IconGift, IconRefresh, IconSparkles, IconTicket } from '@tabler/icons-react';
import { CatalogAdminPanel } from '../../../features/catalog-admin/ui/CatalogAdminPanel';
import { fetchAdminRedemptions, fetchRewards, fetchUserAchievements, fetchUserMissions, fetchUserPoints, fetchUserProgression, fetchUserTransactions, redeemReward, retryRedemption, type AdminRedemption, type PointBalance, type PointTransaction, type Redemption, type Reward, type UserAchievement, type UserMission, type UserProgression } from '../../../shared/api/client';
import { useScope } from '../../../shared/scope/ScopeContext';

type LoyaltyData = {
  balances: PointBalance[];
  transactions: PointTransaction[];
  missions: UserMission[];
  achievements: UserAchievement[];
  progression: UserProgression;
};

export function LoyaltyPage() {
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

  const refresh = useCallback(async (targetUser = userId) => {
    setLoading(true);
    setError(null);
    try {
      if (!scope) {
        setError(scopeError ?? 'Select a tenant environment to inspect loyalty data.');
        return;
      }
      const [points, transactions, missions, achievements, progression, catalog, recentRedemptions] = await Promise.all([
        fetchUserPoints(targetUser),
        fetchUserTransactions(targetUser),
        fetchUserMissions(targetUser),
        fetchUserAchievements(targetUser),
        fetchUserProgression(targetUser),
        fetchRewards(scope),
        fetchAdminRedemptions(8, { tenant_id: scope.tenant_id, environment_id: scope.environment_id }),
      ]);
      setData({ balances: points.balances, transactions: transactions.transactions, missions: missions.missions, achievements: achievements.achievements, progression });
      setLedgerCursor(transactions.next_cursor);
      setRewards(catalog.filter((reward) => reward.active));
      setRedemptions(recentRedemptions);
    } catch (refreshError) {
      setError(refreshError instanceof Error ? refreshError.message : 'Loyalty data could not be loaded.');
    } finally {
      setLoading(false);
    }
  }, [scope, scopeError, userId]);

  useEffect(() => { void refresh(); }, [refresh]);

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
      const redemption = await redeemReward(reward.id, {
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
      const retried = await retryRedemption(redemption.id);
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
      const next = await fetchUserTransactions(userId, 12, ledgerCursor);
      setData((current) => ({ ...current, transactions: [...current.transactions, ...next.transactions] }));
      setLedgerCursor(next.next_cursor);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'More ledger entries could not be loaded.');
    } finally {
      setLedgerLoadingMore(false);
    }
  }

  const totalPoints = useMemo(() => data.balances.filter((balance) => balance.currency !== 'xp').reduce((total, balance) => total + balance.balance, 0), [data.balances]);
  const balanceFor = (currency?: string) => data.balances.find((balance) => balance.currency === currency)?.balance ?? 0;

  return (
    <main className="overview-main loyalty-page">
      <header className="overview-header">
        <div>
          <div className="breadcrumb"><span>Platform</span><span>↗</span><span>Points &amp; rewards</span></div>
          <h1>Make loyalty feel earned.</h1>
          <p>Inspect balances, follow the ledger, and redeem a reward for any scoped user.</p>
        </div>
        <div className="header-actions"><button className="icon-button refresh-button" type="button" onClick={() => void refresh()} aria-label="Refresh points and rewards"><IconRefresh size={17} stroke={1.6} /></button></div>
      </header>

      <section className="loyalty-toolbar panel">
        <form onSubmit={handleUserSubmit}>
          <label><span>Inspect external user</span><div><IconTicket size={15} /><input value={userInput} onChange={(event) => setUserInput(event.target.value)} placeholder="user-42" aria-label="External user ID" /><button className="primary-button" type="submit">Load user</button></div></label>
        </form>
        <div className={`loyalty-context ${scopeLoading ? 'loading' : ''}`}><span className="status-dot" />Live tenant scope<strong>{scope ? `${scope.tenant_id} / ${scope.environment_id}` : scopeLoading ? 'Loading…' : 'Unavailable'}</strong></div>
      </section>

      <CatalogAdminPanel tenant={scope} onChanged={() => void refresh(userId)} />

      {error && <div className="inline-state error-state loyalty-error" role="alert"><strong>Loyalty workspace unavailable</strong><span>{error}</span></div>}

      <section className="loyalty-summary-grid" aria-label="Point summary">
        <div className="loyalty-total"><span className="section-label">{userId} / available</span><strong>{loading ? '—' : totalPoints.toLocaleString()}</strong><span>spendable points across currencies</span></div>
        {data.balances.slice(0, 2).map((balance) => <div className="balance-card" key={balance.currency_id}><div className="balance-card-icon"><IconCoin size={17} /></div><span>{balance.name}</span><strong>{loading ? '—' : balance.balance.toLocaleString()}</strong><small>{balance.currency}</small></div>)}
        {data.balances.length === 0 && !loading && <div className="balance-card balance-empty"><IconCoin size={17} /><span>No balances yet</span><small>The first award will create an account.</small></div>}
        <div className="balance-card progression-balance-card"><div className="balance-card-icon"><IconSparkles size={17} /></div><span>Progression</span><strong>{loading ? '—' : `Level ${data.progression.level}`}</strong><small>{loading ? 'Loading XP' : `${data.progression.total_xp.toLocaleString()} XP · ${data.progression.level_name}`}</small></div>
      </section>

      <div className="loyalty-grid">
        <section className="panel ledger-panel" aria-labelledby="ledger-title">
          <div className="panel-heading"><div><span className="section-label">Append-only record</span><h2 id="ledger-title">Recent ledger</h2></div><span className="execution-count"><IconSparkles size={14} />{data.transactions.length} entries</span></div>
          {loading ? <div className="registry-loading"><div /><div /><div /></div> : data.transactions.length === 0 ? <div className="inline-state empty-state"><IconCoin size={20} /><strong>No transactions yet</strong><span>Point awards and redemptions will appear here for this user.</span></div> : <><ul className="ledger-list">{data.transactions.map((transaction) => <li key={transaction.id}><span className={`ledger-mark ${transaction.amount < 0 ? 'spend' : 'award'}`}>{transaction.amount < 0 ? <IconArrowDownRight size={15} /> : <IconSparkles size={14} />}</span><span className="ledger-copy"><strong>{transaction.entry_type.replaceAll('_', ' ')}</strong><small>{transaction.currency_code} · {new Date(transaction.created_at).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })}</small></span><span className={`ledger-amount ${transaction.amount < 0 ? 'negative' : ''}`}>{transaction.amount > 0 ? '+' : ''}{transaction.amount.toLocaleString()}</span></li>)}</ul>{ledgerCursor && <button className="load-more-button" type="button" onClick={() => void loadMoreLedger()} disabled={ledgerLoadingMore}>{ledgerLoadingMore ? 'Loading…' : 'Load more ledger entries'}</button>}</>}
        </section>

        <section className="panel rewards-panel" aria-labelledby="rewards-title">
          <div className="panel-heading"><div><span className="section-label">Catalog / digital unlocks</span><h2 id="rewards-title">Rewards</h2></div><IconGift size={19} stroke={1.5} /></div>
          {loading ? <div className="registry-loading"><div /><div /></div> : rewards.length === 0 ? <div className="inline-state empty-state"><IconGift size={20} /><strong>Catalog is empty</strong><span>Create a reward from the admin API to make it redeemable.</span></div> : <ul className="reward-list">{rewards.map((reward) => { const balance = balanceFor(reward.cost_currency); const cost = reward.cost_amount ?? 0; const canRedeem = !cost || balance >= cost; return <li className="reward-card" key={reward.id}><div className="reward-card-top"><span className="reward-icon"><IconGift size={16} /></span><span><strong>{reward.name}</strong><small>{reward.description || 'Digital unlock'}</small></span></div><div className="reward-card-bottom"><span>{cost ? `${cost} ${reward.cost_currency}` : 'Free'}</span><button className="text-button accent" type="button" disabled={!canRedeem || redeeming === reward.id} onClick={() => void handleRedeem(reward)}>{redeeming === reward.id ? 'Delivering…' : canRedeem ? 'Redeem' : 'Need points'}</button></div></li>; })}</ul>}
        </section>
      </div>

      <section className="panel progression-panel" aria-labelledby="progression-title">
        <div className="panel-heading"><div><span className="section-label">Engagement / live state</span><h2 id="progression-title">Progression</h2></div><span className="execution-count"><IconSparkles size={14} />{data.missions.length + data.achievements.length} tracks</span></div>
        {loading ? <div className="registry-loading"><div /><div /></div> : <><div className="progression-xp-strip"><span className="progression-label">XP rank</span><strong>{data.progression.level_name} · Level {data.progression.level}</strong><span>{data.progression.total_xp.toLocaleString()} XP{data.progression.next_level_xp ? ` · ${Math.max(0, data.progression.next_level_xp - data.progression.total_xp).toLocaleString()} to next level` : ' · top level reached'}</span></div>{data.missions.length === 0 && data.achievements.length === 0 ? <div className="inline-state empty-state"><IconSparkles size={20} /><strong>No progression tracks yet</strong><span>Create a mission or achievement from the admin API to start tracking user behavior.</span></div> : <div className="progression-columns"><div><span className="progression-label">Missions</span>{data.missions.length === 0 ? <p className="progression-empty">No missions configured.</p> : <ul className="progression-list">{data.missions.map((mission) => <li key={mission.id}><span className={`progression-mark ${mission.completed ? 'complete' : ''}`}>{mission.completed ? <IconCircleCheck size={14} /> : <IconSparkles size={14} />}</span><span className="progression-copy"><strong>{mission.name}</strong><small>{mission.progress} / {mission.target_count} · {mission.event_type}</small></span><span className="progression-status">{mission.completed ? 'complete' : `${Math.round((mission.progress / mission.target_count) * 100)}%`}</span></li>)}</ul>}</div><div><span className="progression-label">Achievements</span>{data.achievements.length === 0 ? <p className="progression-empty">No achievements configured.</p> : <ul className="progression-list">{data.achievements.map((achievement) => <li key={achievement.id}><span className={`progression-mark ${achievement.unlocked ? 'complete' : ''}`}>{achievement.unlocked ? <IconCircleCheck size={14} /> : <IconSparkles size={14} />}</span><span className="progression-copy"><strong>{achievement.name}</strong><small>{achievement.progress} / {achievement.required_count} · {achievement.event_type}</small></span><span className="progression-status">{achievement.unlocked ? 'unlocked' : `${Math.round((achievement.progress / achievement.required_count) * 100)}%`}</span></li>)}</ul>}</div></div>}</>}
      </section>

      <section className="panel redemptions-panel" aria-labelledby="redemptions-title">
        <div className="panel-heading"><div><span className="section-label">Operations / provider state</span><h2 id="redemptions-title">Recent redemptions</h2></div><span className="execution-count"><IconGift size={14} />{redemptions.length} records</span></div>
        {loading ? <div className="registry-loading"><div /><div /></div> : redemptions.length === 0 ? <div className="inline-state empty-state"><IconGift size={20} /><strong>No redemptions yet</strong><span>Successful and failed reward deliveries will appear here.</span></div> : <ul className="redemption-list">{redemptions.map((redemption) => <li key={redemption.id}><span className={`redemption-state ${redemption.status === 'fulfilled' ? 'fulfilled' : redemption.status === 'failed' ? 'failed' : ''}`}><span className="status-dot" />{redemption.status}</span><span className="redemption-copy"><strong>{redemption.reward_name}</strong><small>{redemption.external_user_id} · {new Date(redemption.created_at).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })} · {redemption.attempt_count ?? 0} attempts</small>{redemption.status === 'delivering' && <small>{redemption.reconciliation_attempts ?? 0} reconciliation checks{redemption.last_provider_check_error ? ` · ${redemption.last_provider_check_error}` : ''}</small>}</span><span className="redemption-reference">{redemption.provider_reference ?? redemption.id}</span>{redemption.status === 'failed' && redemption.external_user_id === userId && <button className="text-button accent" type="button" onClick={() => void handleRetry(redemption)} disabled={retrying === redemption.id}>{retrying === redemption.id ? 'Retrying…' : 'Retry'}</button>}</li>)}</ul>}
      </section>

      {lastRedemption && <section className="redemption-receipt" aria-live="polite"><span className="receipt-icon"><IconCircleCheck size={15} /></span><span><strong>{lastRedemption.status === 'fulfilled' ? 'Reward fulfilled' : `Redemption ${lastRedemption.status}`}</strong><small>{lastRedemption.provider_reference ?? lastRedemption.id}</small></span><span className="receipt-status"><span className="status-dot" />provider delivery</span></section>}
    </main>
  );
}
