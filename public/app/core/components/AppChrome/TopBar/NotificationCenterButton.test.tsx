import { HttpResponse, http } from 'msw';
import { act, render, screen } from 'test/test-utils';

import { AppEvents } from '@grafana/data';
import { setBackendSrv } from '@grafana/runtime';
import { FlagKeys } from '@grafana/runtime/internal';
import { setupMockServer } from '@grafana/test-utils/server';
import { setTestFlags } from '@grafana/test-utils/unstable';
import { appEvents } from 'app/core/app_events';
import { backendSrv } from 'app/core/services/backend_srv';
import { contextSrv } from 'app/core/services/context_srv';

import { NotificationCenterButton } from './NotificationCenterButton';

setBackendSrv(backendSrv);
const server = setupMockServer();

interface NotificationFixture {
  id: string;
  title: string;
  body: string;
  created: string;
  read: boolean;
}

function renderBell() {
  return render(
    <div>
      <span>Top bar</span>
      <NotificationCenterButton />
    </div>
  );
}

describe('NotificationCenterButton', () => {
  const signedIn = contextSrv.user.isSignedIn;

  beforeEach(() => {
    contextSrv.user.isSignedIn = true;
    setTestFlags({ [FlagKeys.GrafanaNotificationCenter]: true });
  });

  afterEach(async () => {
    contextSrv.user.isSignedIn = signedIn;
    await act(async () => {
      setTestFlags({});
    });
  });

  it('hides the bell when the notification center toggle is off', () => {
    setTestFlags({ [FlagKeys.GrafanaNotificationCenter]: false });

    renderBell();

    expect(screen.getByText('Top bar')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Notifications/ })).not.toBeInTheDocument();
  });

  it('shows the unread count on the bell', async () => {
    server.use(http.get('/api/user/notifications/unread-count', () => HttpResponse.json({ count: 3 })));

    renderBell();

    expect(await screen.findByRole('button', { name: 'Notifications, 3 unread' })).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
  });

  it('hides the badge when there are no unread notifications', async () => {
    server.use(http.get('/api/user/notifications/unread-count', () => HttpResponse.json({ count: 0 })));

    renderBell();

    expect(await screen.findByRole('button', { name: 'Notifications' })).toBeInTheDocument();
    expect(screen.queryByText('0')).not.toBeInTheDocument();
  });

  it('caps the unread badge at 99+', async () => {
    server.use(http.get('/api/user/notifications/unread-count', () => HttpResponse.json({ count: 120 })));

    renderBell();

    expect(await screen.findByRole('button', { name: 'Notifications, 120 unread' })).toBeInTheDocument();
    expect(screen.getByText('99+')).toBeInTheDocument();
    expect(screen.queryByText('120')).not.toBeInTheDocument();
  });

  it('lists notifications and marks one as read', async () => {
    let notifications: NotificationFixture[] = [
      {
        id: 'n1',
        title: 'Deploy finished',
        body: 'API deploy succeeded',
        created: '2026-03-02T12:00:00.000Z',
        read: false,
      },
      {
        id: 'n2',
        title: 'Quota warning',
        body: 'Org is near the limit',
        created: '2026-03-01T12:00:00.000Z',
        read: false,
      },
    ];
    const unread = () => notifications.filter((item) => !item.read).length;

    server.use(
      http.get('/api/user/notifications/unread-count', () => HttpResponse.json({ count: unread() })),
      http.get('/api/user/notifications', () => HttpResponse.json({ notifications })),
      http.post('/api/user/notifications/read', async ({ request }) => {
        const body = (await request.json()) as { id?: string; all?: boolean };
        notifications = notifications.map((item) => (body.all || item.id === body.id ? { ...item, read: true } : item));
        return HttpResponse.json({ count: unread() });
      })
    );

    const { user } = renderBell();

    expect(await screen.findByText('2')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Notifications, 2 unread' }));

    expect(await screen.findByRole('dialog', { name: 'Notifications' })).toBeInTheDocument();
    expect(screen.getByText('Deploy finished')).toBeInTheDocument();
    expect(screen.getByText('API deploy succeeded')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Mark all as read' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Mark Deploy finished as read' }));

    // The open menu marks its trigger aria-hidden, so include hidden nodes when reading the badge.
    expect(await screen.findByRole('button', { name: 'Notifications, 1 unread', hidden: true })).toBeInTheDocument();
    expect(screen.getByText('1', { hidden: true })).toBeInTheDocument();
    expect(screen.getByText('Deploy finished')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Mark Quota warning as read' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Mark Deploy finished as read' })).not.toBeInTheDocument();
  });

  it('shows a loading state and then an empty state', async () => {
    let releaseList: (response: Response) => void = () => {};
    const pendingList = new Promise<Response>((resolve) => {
      releaseList = resolve;
    });
    server.use(
      http.get('/api/user/notifications/unread-count', () => HttpResponse.json({ count: 0 })),
      http.get('/api/user/notifications', () => pendingList)
    );

    const { user } = renderBell();
    await user.click(await screen.findByRole('button', { name: 'Notifications' }));

    expect(await screen.findByRole('status')).toHaveTextContent('Loading notifications');

    releaseList(HttpResponse.json({ notifications: [] }));

    expect(await screen.findByText('No notifications')).toBeInTheDocument();
    expect(screen.queryByText('Loading notifications')).not.toBeInTheDocument();
  });

  it('shows an error when notifications fail to load', async () => {
    const emit = jest.spyOn(appEvents, 'emit');
    server.use(
      http.get('/api/user/notifications/unread-count', () => HttpResponse.json({ count: 1 })),
      http.get('/api/user/notifications', () => HttpResponse.json({ message: 'database unavailable' }, { status: 500 }))
    );

    const { user } = renderBell();
    await user.click(await screen.findByRole('button', { name: 'Notifications, 1 unread' }));

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Failed to load notifications');
    expect(alert).toHaveTextContent('database unavailable');
    expect(emit).toHaveBeenCalledWith(AppEvents.alertError, ['Failed to load notifications', 'database unavailable']);
    emit.mockRestore();
  });

  it('renders notification text as text', async () => {
    server.use(
      http.get('/api/user/notifications/unread-count', () => HttpResponse.json({ count: 1 })),
      http.get('/api/user/notifications', () =>
        HttpResponse.json({
          notifications: [
            {
              id: 'xss',
              title: '<script>alert(1)</script>',
              body: '<img src=x onerror=alert(1)>',
              created: '2026-03-02T12:00:00.000Z',
              read: false,
            },
          ],
        })
      )
    );

    const { user } = renderBell();
    await user.click(await screen.findByRole('button', { name: 'Notifications, 1 unread' }));

    const dialog = await screen.findByRole('dialog', { name: 'Notifications' });
    expect(screen.getByText('<script>alert(1)</script>')).toBeInTheDocument();
    expect(screen.getByText('<img src=x onerror=alert(1)>')).toBeInTheDocument();
    expect(dialog.querySelector('img')).not.toBeInTheDocument();
    expect(dialog.querySelector('script')).not.toBeInTheDocument();
  });
});
