<?php

class Document extends Theme {

	protected $pageTitle = 'Dashboard';
	private $db = null;

	public function __construct(&$main, &$twig, $vars) {

		$vars['page_title'] = $this->pageTitle;

		$this->db = $main->getDB();

		$q = $this->db->query("SELECT SUM(deposit_value) AS deposit_total,
								SUM(asset_value) AS asset_total,
								SUM(asset_value - deposit_value) AS gain,
								DATE_FORMAT(epoch, '%b %Y') AS period,
								EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth
								FROM asset_log
								GROUP BY period, yearMonth
								ORDER BY yearMonth ASC");

		while ($period = $this->db->fetch($q)) {
			// Calculate Gain Difference
			$period['delta'] = 0;
			if (isset($last)) {
				$period['delta'] = ($period['gain'] - $last['gain']);
			}

			$vars['periodData'][] = $period;
			$last = $period;
		}

		$last['deposit_str'] = $this->prettify($last['deposit_total']);
		$last['asset_str'] = $this->prettify($last['asset_total']);
		$last['gain_str'] = $this->prettify($last['gain']);
		$vars['mostRecent'] = $last;

		$this->vars = $vars;
		$this->document = $twig->load('dashboard.html');
	}

	private function prettify($value) {
		return ($value < 0 ? '-£'.number_format($value * -1, 2) : '£'.number_format($value, 2));
	}

}
//TODO Deal with no results (Fresh install?)
//TODO fill in missing months as zero
