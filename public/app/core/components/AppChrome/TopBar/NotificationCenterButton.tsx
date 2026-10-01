import { css } from '@emotion/css';
import { useCallback, useEffect, useRef, useState } from 'react';

import { AppEvents, type GrafanaTheme2 } from '@grafana/data';
import { t } from '@grafana/i18n';
import { getBackendSrv } from '@grafana/runtime';
import { useFlagGrafanaNotificationCenter } from '@grafana/runtime/internal';
import { Alert, Button, Dropdown, Spinner, Stack, Text, ToolbarButton, useStyles2 } from '@grafana/ui';
import { appEvents } from 'app/core/app_events';
import { contextSrv } from 'app/core/services/context_srv';
import { getMessageFromError } from 'app/core/utils/errors';

const UNREAD_CAP = 99;

interface UserNotification {
  id: string;
  title: string;
  body: string;
  created: string;
  read: boolean;
}

interface NotificationListResponse {
  notifications: UserNotification[];
}

interface UnreadCountResponse {
  count: number;
}

export function NotificationCenterButton() {
  const enabled = useFlagGrafanaNotificationCenter();
  if (!enabled || !contextSrv.user.isSignedIn) {
    return null;
  }
  return <NotificationCenter />;
}

function NotificationCenter() {
  const styles = useStyles2(getStyles);
  const [unread, setUnread] = useState<number | undefined>();
  const [refreshToken, setRefreshToken] = useState(0);

  const refreshUnread = useCallback(() => {
    setRefreshToken((value) => value + 1);
  }, []);

  useEffect(() => {
    let cancelled = false;
    getBackendSrv()
      .get<UnreadCountResponse>('/api/user/notifications/unread-count', undefined, undefined, {
        showErrorAlert: false,
      })
      .then((response) => {
        if (!cancelled) {
          setUnread(response.count);
        }
      })
      .catch((error: unknown) => {
        if (cancelled) {
          return;
        }
        setUnread(undefined);
        appEvents.emit(AppEvents.alertError, [
          t('navigation.notification-center.count-error', 'Failed to load notification count'),
          getMessageFromError(error),
        ]);
      });
    return () => {
      cancelled = true;
    };
  }, [refreshToken]);

  const badge = formatUnreadCount(unread);
  const label =
    unread !== undefined && unread > 0
      ? t('navigation.notification-center.aria-label-unread', '', {
          count: unread,
          defaultValue_one: 'Notifications, {{count}} unread',
          defaultValue_other: 'Notifications, {{count}} unread',
        })
      : t('navigation.notification-center.aria-label', 'Notifications');

  return (
    <div className={styles.anchor}>
      <Dropdown overlay={() => <NotificationPanel onChanged={refreshUnread} />} placement="bottom-end">
        <ToolbarButton
          icon="bell"
          iconOnly
          aria-label={label}
          tooltip={t('navigation.notification-center.tooltip', 'Notifications')}
        />
      </Dropdown>
      {badge !== undefined && (
        <span className={styles.badge} aria-hidden="true">
          {badge}
        </span>
      )}
    </div>
  );
}

function NotificationPanel({ onChanged }: { onChanged: () => void }) {
  const styles = useStyles2(getStyles);
  const [notifications, setNotifications] = useState<UserNotification[] | undefined>();
  const [error, setError] = useState<string | undefined>();
  const [markError, setMarkError] = useState<string | undefined>();
  const requestId = useRef(0);

  const load = useCallback(() => {
    const id = requestId.current + 1;
    requestId.current = id;
    setError(undefined);
    getBackendSrv()
      .get<NotificationListResponse>('/api/user/notifications', undefined, undefined, { showErrorAlert: false })
      .then((response) => {
        if (requestId.current === id) {
          setNotifications(response.notifications);
        }
      })
      .catch((loadError: unknown) => {
        if (requestId.current !== id) {
          return;
        }
        const message = getMessageFromError(loadError);
        setNotifications([]);
        setError(message);
        appEvents.emit(AppEvents.alertError, [
          t('navigation.notification-center.load-error', 'Failed to load notifications'),
          message,
        ]);
      });
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const markRead = async (body: { id: string } | { all: true }) => {
    setMarkError(undefined);
    try {
      await getBackendSrv().post('/api/user/notifications/read', body, { showErrorAlert: false });
      onChanged();
      load();
    } catch (markReadError) {
      const message = getMessageFromError(markReadError);
      setMarkError(message);
      appEvents.emit(AppEvents.alertError, [
        t('navigation.notification-center.mark-error', 'Failed to mark notifications as read'),
        message,
      ]);
    }
  };

  const hasUnread = notifications?.some((item) => !item.read) ?? false;

  return (
    // Clicks inside the panel must not bubble to the menu, which closes on any click.
    // Keyboard users activate the buttons in the panel directly.
    // eslint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-noninteractive-element-interactions
    <div
      className={styles.panel}
      role="dialog"
      aria-label={t('navigation.notification-center.title', 'Notifications')}
      onClick={(event) => event.stopPropagation()}
    >
      <Stack justifyContent="space-between" alignItems="center">
        <Text variant="h5">{t('navigation.notification-center.title', 'Notifications')}</Text>
        {hasUnread && (
          <Button size="sm" fill="text" variant="secondary" onClick={() => markRead({ all: true })}>
            {t('navigation.notification-center.mark-all', 'Mark all as read')}
          </Button>
        )}
      </Stack>
      {notifications === undefined && (
        <div className={styles.status} role="status">
          <Spinner />
          <span>{t('navigation.notification-center.loading', 'Loading notifications')}</span>
        </div>
      )}
      {error && (
        <Alert severity="error" title={t('navigation.notification-center.load-error', 'Failed to load notifications')}>
          {error}
        </Alert>
      )}
      {markError && (
        <Alert
          severity="error"
          title={t('navigation.notification-center.mark-error', 'Failed to mark notifications as read')}
        >
          {markError}
        </Alert>
      )}
      {notifications !== undefined && !error && notifications.length === 0 && (
        <div className={styles.empty} role="status">
          {t('navigation.notification-center.empty', 'No notifications')}
        </div>
      )}
      {notifications !== undefined && notifications.length > 0 && (
        <ul className={styles.list}>
          {notifications.map((item) => (
            <li key={item.id} className={styles.item}>
              <span className={item.read ? styles.readTitle : styles.title}>{item.title}</span>
              <span className={styles.body}>{item.body}</span>
              {!item.read && (
                <Button
                  size="sm"
                  fill="text"
                  variant="secondary"
                  aria-label={t('navigation.notification-center.mark-one', 'Mark {{title}} as read', {
                    title: item.title,
                  })}
                  onClick={() => markRead({ id: item.id })}
                >
                  {t('navigation.notification-center.mark-one-label', 'Mark as read')}
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function formatUnreadCount(count: number | undefined): string | undefined {
  if (count === undefined || count <= 0) {
    return undefined;
  }
  if (count > UNREAD_CAP) {
    return `${UNREAD_CAP}+`;
  }
  return String(count);
}

const getStyles = (theme: GrafanaTheme2) => ({
  anchor: css({
    position: 'relative',
    display: 'inline-flex',
  }),
  badge: css({
    position: 'absolute',
    top: 0,
    right: 0,
    minWidth: theme.spacing(2),
    height: theme.spacing(2),
    padding: theme.spacing(0, 0.5),
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.error.main,
    color: theme.colors.error.contrastText,
    fontSize: theme.typography.size.xs,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: theme.spacing(2),
    textAlign: 'center',
    pointerEvents: 'none',
    whiteSpace: 'nowrap',
  }),
  panel: css({
    width: theme.spacing(45),
    maxWidth: 'calc(100vw - 16px)',
    maxHeight: theme.spacing(60),
    overflow: 'auto',
    background: theme.colors.background.primary,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    boxShadow: theme.shadows.z3,
    padding: theme.spacing(1.5),
  }),
  list: css({
    listStyle: 'none',
    margin: 0,
    padding: 0,
  }),
  item: css({
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'flex-start',
    gap: theme.spacing(0.5),
    padding: theme.spacing(1, 0),
    borderTop: `1px solid ${theme.colors.border.weak}`,
  }),
  title: css({
    fontWeight: theme.typography.fontWeightMedium,
    overflowWrap: 'anywhere',
  }),
  readTitle: css({
    color: theme.colors.text.secondary,
    overflowWrap: 'anywhere',
  }),
  body: css({
    color: theme.colors.text.secondary,
    overflowWrap: 'anywhere',
  }),
  status: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(2, 0),
  }),
  empty: css({
    padding: theme.spacing(2, 0),
    color: theme.colors.text.secondary,
  }),
});
