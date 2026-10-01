import { render, screen } from 'test/test-utils';

import { store } from '@grafana/data';
import { interceptLinkClicks } from 'app/core/navigation/patch/interceptLinkClicks';

import { ctaClicked } from '../analytics/main';

import { LEARN_TIPS_HIDDEN_KEY, LearningTips } from './LearningTips';

jest.mock('../analytics/main', () => ({
  ctaClicked: jest.fn(),
}));

const STARTING_LINKS: Array<[string, string]> = [
  ['Add a data source', '/connections/datasources'],
  ['Open Explore', '/explore'],
  ['New dashboard', '/dashboard/new'],
  ['View alert rules', '/alerting/list'],
];

describe('LearningTips', () => {
  beforeEach(() => {
    window.localStorage.clear();
    jest.mocked(ctaClicked).mockClear();
    document.addEventListener('click', interceptLinkClicks);
  });

  afterEach(() => {
    document.removeEventListener('click', interceptLinkClicks);
  });

  it('links the four starting steps and the getting started guide', () => {
    render(<LearningTips />);

    expect(screen.getByRole('heading', { name: 'Start here' })).toBeInTheDocument();
    for (const [name, href] of STARTING_LINKS) {
      expect(screen.getByRole('link', { name })).toHaveAttribute('href', href);
    }
    expect(screen.getByRole('link', { name: 'Getting started guide' })).toHaveAttribute(
      'href',
      'https://grafana.com/docs/grafana/latest/getting-started/'
    );
  });

  it('hides the steps and restores them from Show tips', async () => {
    const { user } = render(<LearningTips />);

    await user.click(screen.getByRole('button', { name: 'Hide tips' }));

    expect(screen.getByText('Starting tips are hidden.')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Open Explore' })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Show tips' }));

    expect(screen.getByRole('link', { name: 'Open Explore' })).toHaveAttribute('href', '/explore');
  });

  it('stays hidden when the stored preference is set', () => {
    store.set(LEARN_TIPS_HIDDEN_KEY, true);

    render(<LearningTips />);

    expect(screen.getByRole('button', { name: 'Show tips' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Start here' })).not.toBeInTheDocument();
  });

  it('records which starting step was opened', async () => {
    const { user } = render(<LearningTips />);

    await user.click(screen.getByRole('link', { name: 'Open Explore' }));

    expect(jest.mocked(ctaClicked)).toHaveBeenCalledWith({
      surface: 'learn_tips',
      action: 'open_explore',
      placement: 'card',
    });
  });
});
