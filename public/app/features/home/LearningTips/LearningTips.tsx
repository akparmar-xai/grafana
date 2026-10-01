import { css } from '@emotion/css';

import { type GrafanaTheme2, type IconName } from '@grafana/data';
import { t } from '@grafana/i18n';
import { Button, Icon, LinkButton, Stack, Text, useStyles2 } from '@grafana/ui';
import { useStoredBoolean } from 'app/core/hooks/useStored';

import { FooterAction, FooterActions } from '../FooterActions';
import { HomeGrid } from '../HomeGrid';
import { HomeSection } from '../HomeSection';
import { ctaClicked } from '../analytics/main';

export const LEARN_TIPS_HIDDEN_KEY = 'grafana.home.learn-tips.hidden';

const DOCS_HREF = 'https://grafana.com/docs/grafana/latest/getting-started/';

type LearnTipAction = 'open_datasources' | 'open_explore' | 'open_dashboard' | 'open_alerting';

interface LearnTip {
  id: string;
  step: number;
  title: string;
  description: string;
  cta: string;
  href: string;
  icon: IconName;
  action: LearnTipAction;
}

function getTips(): LearnTip[] {
  return [
    {
      id: 'connect-data',
      step: 1,
      title: t('home.learn-tips.cards.connect-data.title', 'Connect a data source'),
      description: t(
        'home.learn-tips.cards.connect-data.description',
        'Grafana queries data that lives somewhere else. Add Prometheus, Loki, or the built-in TestData source. Dashboards and alerts both run against it.'
      ),
      cta: t('home.learn-tips.cards.connect-data.cta', 'Add a data source'),
      href: '/connections/datasources',
      icon: 'database',
      action: 'open_datasources',
    },
    {
      id: 'explore',
      step: 2,
      title: t('home.learn-tips.cards.explore.title', 'Run a query in Explore'),
      description: t(
        'home.learn-tips.cards.explore.description',
        'Explore is a scratch pad. Pick the data source, run a query, and move the time range until the result looks right. Nothing is saved until you decide to keep it.'
      ),
      cta: t('home.learn-tips.cards.explore.cta', 'Open Explore'),
      href: '/explore',
      icon: 'compass',
      action: 'open_explore',
    },
    {
      id: 'dashboard',
      step: 3,
      title: t('home.learn-tips.cards.dashboard.title', 'Save a dashboard'),
      description: t(
        'home.learn-tips.cards.dashboard.description',
        'A dashboard is the page you share: panels, a time picker, and optional variables such as a host or namespace. Add a panel from the query you tried, then save.'
      ),
      cta: t('home.learn-tips.cards.dashboard.cta', 'New dashboard'),
      href: '/dashboard/new',
      icon: 'dashboard',
      action: 'open_dashboard',
    },
    {
      id: 'alerting',
      step: 4,
      title: t('home.learn-tips.cards.alerting.title', 'Alert when a value is wrong'),
      description: t(
        'home.learn-tips.cards.alerting.description',
        'An alert rule repeats a query and fires when the result crosses a condition. Firing alerts show up on this page. The rules themselves live under Alerting.'
      ),
      cta: t('home.learn-tips.cards.alerting.cta', 'View alert rules'),
      href: '/alerting/list',
      icon: 'bell',
      action: 'open_alerting',
    },
  ];
}

/** First-run orientation for the classic homepage: what Grafana is, then the four steps to a useful stack. */
export function LearningTips() {
  const styles = useStyles2(getStyles);
  const [hidden, setHidden] = useStoredBoolean(LEARN_TIPS_HIDDEN_KEY, false);
  const tips = getTips();

  if (hidden) {
    return (
      <HomeSection>
        <Stack direction="row" alignItems="center" justifyContent="space-between" gap={2}>
          <Text color="secondary" variant="body">
            {t('home.learn-tips.hidden', 'Starting tips are hidden.')}
          </Text>
          <Button type="button" variant="secondary" fill="text" size="sm" onClick={() => setHidden(false)}>
            {t('home.learn-tips.show', 'Show tips')}
          </Button>
        </Stack>
      </HomeSection>
    );
  }

  return (
    <HomeSection>
      <Stack direction="column" gap={2}>
        <Stack direction="row" alignItems="flex-start" justifyContent="space-between" gap={2}>
          <Stack direction="column" gap={0.5}>
            <Text element="h2" variant="h4">
              {t('home.learn-tips.title', 'Start here')}
            </Text>
            <Text color="secondary" variant="body">
              {t(
                'home.learn-tips.intro',
                'Grafana shows metrics, logs, and traces that live somewhere else. Connect that data, query it, and save the view as a dashboard. Do these four steps in order. Below them are dashboards you have opened, alerts that are firing, and the Grafana blog.'
              )}
            </Text>
          </Stack>
          <Button type="button" variant="secondary" fill="text" size="sm" onClick={() => setHidden(true)}>
            {t('home.learn-tips.hide', 'Hide tips')}
          </Button>
        </Stack>

        <HomeGrid columns={2} gap={2}>
          {tips.map((tip) => (
            <div key={tip.id} className={styles.tip}>
              <Stack direction="column" gap={1.5} justifyContent="space-between" flex={1}>
                <Stack direction="column" gap={1}>
                  <Stack direction="row" gap={1} alignItems="center">
                    <span className={styles.step} aria-hidden="true">
                      {tip.step}
                    </span>
                    <Icon name={tip.icon} />
                    <Text element="h3" variant="h5">
                      {tip.title}
                    </Text>
                  </Stack>
                  <Text color="secondary" variant="body">
                    {tip.description}
                  </Text>
                </Stack>
                <div>
                  <LinkButton
                    variant="secondary"
                    fill="outline"
                    size="sm"
                    href={tip.href}
                    icon="arrow-right"
                    iconPlacement="right"
                    onClick={() =>
                      ctaClicked({
                        surface: 'learn_tips',
                        action: tip.action,
                        placement: 'card',
                      })
                    }
                  >
                    {tip.cta}
                  </LinkButton>
                </div>
              </Stack>
            </div>
          ))}
        </HomeGrid>

        <FooterActions>
          <FooterAction
            icon="book"
            href={DOCS_HREF}
            external
            onClick={() =>
              ctaClicked({
                surface: 'learn_tips',
                action: 'open_docs',
                placement: 'footer',
              })
            }
          >
            {t('home.learn-tips.docs', 'Getting started guide')}
          </FooterAction>
        </FooterActions>
      </Stack>
    </HomeSection>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  tip: css({
    display: 'flex',
    minWidth: 0,
    padding: theme.spacing(2),
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.border.weak}`,
    background: theme.colors.background.secondary,
  }),
  step: css({
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
    width: theme.spacing(3),
    height: theme.spacing(3),
    borderRadius: theme.shape.radius.circle,
    background: theme.colors.primary.transparent,
    color: theme.colors.primary.text,
    fontSize: theme.typography.bodySmall.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
  }),
});
