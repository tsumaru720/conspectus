<?php

class Document extends Theme {

    protected $pageTitle = 'Projections';
    private $db = null;
    private $entityManager = null;

    public function __construct(&$main, &$twig, $vars) {

        $vars['page_title'] = $this->pageTitle;

        $this->db = $main->getDB();
        $this->entityManager = $main->getEntityManager();

        if ($vars['left_menu'] != 'all') {
            $vars['modifier'] = "=";
        } else {
            $vars['item_id'] = 0;
            $vars['modifier'] = ">";
        }
        $data = array(':item_id' => $vars['item_id']);

        if ($vars['type'] == 'asset') {
            $dataQuery = $this->db->query("SELECT
                                            SUM(deposit_value) AS deposit_total,
                                            SUM(asset_value) AS asset_total,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth,
                                            EXTRACT(YEAR FROM epoch) AS year
                                        FROM
                                            asset_log
                                        WHERE
                                            asset_id ".$vars['modifier']." :item_id
                                        GROUP BY
                                            period,
                                            yearMonth,
                                            year
                                        ORDER BY
                                            yearMonth ASC", $data);
            if ($vars['item_id'] > 0) {
                $entity = $this->entityManager->getAsset($vars['item_id']);
                $this->pageTitle = "Projections - ".$entity->getDescription();
                $vars['page_title'] = $entity->getDescription();
                $vars['asset_class'] = $entity->getClass();
                $vars['asset_class_id'] = $entity->getClassID();
                $vars['closed'] = $entity->isClosed();
            }
        } elseif ($vars['type'] == 'class') {
            $dataQuery = $this->db->query("SELECT
                                            SUM(deposit_value) AS deposit_total,
                                            SUM(asset_value) AS asset_total,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth,
                                            EXTRACT(YEAR FROM epoch) AS year
                                        FROM
                                            asset_log
                                        LEFT JOIN asset_list ON asset_log.asset_id = asset_list.id
                                        LEFT JOIN asset_classes ON asset_classes.id = asset_list.asset_class
                                        WHERE
                                            asset_classes.id ".$vars['modifier']." :item_id
                                        GROUP BY
                                            period,
                                            yearMonth,
                                            year
                                        ORDER BY
                                            yearMonth ASC", $data);
            if ($vars['item_id'] > 0) {
                $entity = $this->entityManager->getClass($vars['item_id']);
                $this->pageTitle = "Projections - ".$entity->getDescription();
                $vars['page_title'] = $entity->getDescription();
            }
        }

        $startValue = 0;
        $startYear = 0;
        $curYear = date("Y");

        $endYear = "2048";
        $endValue = "1000000";
        //TODO: make these optional

        while ($period = $this->db->fetch($dataQuery)) {
            $year = $period['year'];
            if ($startValue == 0) {
                $startYear = $year;
                $startValue = $period['asset_total'];
                $vars['startYear'] = $startYear;
                $vars['startValue'] = $startValue;
            }
            $vars['actual']['data'][$year]['year'] = $year;
            $vars['actual']['data'][$year]['value'] = $period['asset_total'];
        }

        // Account for no historic data
        if (!array_key_exists('actual', $vars)) {
            $year = $curYear;
            $startYear = $year;
            $startValue = 0;
            $vars['actual']['data'][$year]['year'] = $year;
            $vars['actual']['data'][$year]['value'] = 0;
            $vars['startYear'] = $startYear;
            $vars['startValue'] = $startValue;
        }

        $displayYears = ($curYear + 10) - $startYear;

        $previousYear = $curYear - 1;
        if (!array_key_exists($previousYear, $vars['actual']['data'])) {
            $previousYear = $curYear;
        }

        $vars['initial'] = $this->seedTargets($startYear, $endYear, $startValue, $endValue, $displayYears);
        $vars['revised'] = $this->seedTargets($curYear, $endYear, $vars['actual']['data'][$previousYear]['value'], $endValue, $displayYears);

        if (array_key_exists($curYear, $vars['actual']['data'])) {
            if ($vars['revised']['data'][$curYear]['value'] < $vars['actual']['data'][$curYear]['value']) {
                $vars['revised']['data'][$curYear]['value'] = 0;
            } else {
                $vars['revised']['data'][$curYear]['value'] = $vars['revised']['data'][$curYear]['value'] - $vars['actual']['data'][$curYear]['value'];
            }
        }

        $revised_seed = array_fill(0, ($curYear - $startYear), 0);
        $vars['revised']['data'] = array_merge($revised_seed, $vars['revised']['data']);

//var_dump($vars);

        $this->vars = $vars;
        $this->document = $twig->load('projections.html');
    }

    private function seedTargets($startYear, $endYear, $startValue, $endValue, $returnAmount = 0) {
        $years = $endYear - $startYear;

        if ($startValue != 0) {
            $percent = pow(($endValue / $startValue), (1 / $years));
        } else {
            $percent = 1;
        }
        $targets['percent'] = $percent;
        $targets['percent_str'] = number_format(($percent - 1) * 100, 2);

        if ($returnAmount == 0) {
            $returnAmount = $years;
        }

        $value = $startValue;
        for ($i = 0; $i < $years; $i++) {
            if ($i == $returnAmount) { break; }
            $value = $value * $percent;
            $targets['data'][$startYear + $i]['year'] = $startYear + $i;
            $targets['data'][$startYear + $i]['value'] = number_format($value, 2, '.', '');
        }
        return $targets;
    }

}
//TODO Deal with no results (Fresh install?)
//TODO fill in missing months as zero
