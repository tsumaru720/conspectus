<?php

class Document extends Theme {

	protected $pageTitle = 'Breakdown';
	private $db = null;

	public function __construct(&$main, &$twig, $vars) {

		$vars['page_title'] = $this->pageTitle;

		$this->db = $main->getDB();
		$page = $main->getPage();

		$logQuery = $this->db->query("SELECT asset_id, 
								description,
								deposit_value,
								asset_value,
								(asset_value - deposit_value) AS gain,
								DATE_FORMAT(epoch, '%b %Y') AS period,
								EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth
								FROM asset_log
								LEFT JOIN asset_list
								ON asset_log.asset_id = asset_list.id
								ORDER BY yearMonth ASC, description ASC");

		while ($log = $this->db->fetch($logQuery)) {
			$vars['labels'][$log['yearMonth']] = $log['period'];

			$vars['log'][$log['asset_id']]['name'] = $log['description'];
			$vars['log'][$log['asset_id']]['data'][$log['yearMonth']] = $log['asset_value'];
			
			$last['log'][$log['asset_id']]['name'] = $log['description'];
			$last['log'][$log['asset_id']]['value'] = $log['asset_value'];
		}

		usort($last['log'], function($a, $b) {
			return $b['value'] - $a['value'];
		});
		$vars['mostRecent'] = $last['log'];

		foreach ($vars['log'] as $key => $entry) {
			foreach ($vars['labels'] as $period => $v) {
				if (!array_key_exists($period, $entry['data'])) {
					$vars['log'][$key]['data'][$period] = 0;
					ksort($vars['log'][$key]['data']);
				}
			}
		}

		$this->vars = $vars;
		$this->document = $twig->load('breakdown.html');
	}

}
