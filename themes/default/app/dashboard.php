<?php

class Document extends Theme {

	protected $pageTitle = 'Dashboard';
	private $db = null;

	public function __construct(&$main, &$twig, $vars) {

		$vars['page_title'] = $this->pageTitle;

		$this->db = $main->getDB();

		$q = $this->db->query("SELECT SUM(deposit_value) AS deposit_total, SUM(asset_value) AS asset_total, SUM(asset_value - deposit_value) AS gain, DATE_FORMAT(epoch, '%b %Y') AS period, EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth FROM asset_log GROUP BY period, yearMonth ORDER BY yearMonth ASC");
		while ($period = $this->db->fetch($q)) {
			$period['deposit_str'] = '£'.number_format($period['deposit_total'], 2);
			$period['asset_str'] = ($period['asset_total'] < 0 ? '-£'.number_format($period['asset_total'] * -1,2) : '£'.number_format($period['asset_total'],2));
			$period['gain_str'] = ($period['gain'] < 0 ? '-£'.number_format($period['gain'] * -1,2) : '£'.number_format($period['gain'],2));
			$vars['periodData'][] = $period;
			$last = $period;
		}
		$vars['mostRecent'] = $last;
		$this->debug($last);
		$this->vars = $vars;
		$this->document = $twig->load('dashboard.html');
	}

}
//TODO Deal with no results (Fresh install?)
//TODO fill in missing months as zero
