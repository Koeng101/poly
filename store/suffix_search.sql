WITH recursive suffix_search(search_string, compare_string, pos, upper_limit, lower_limit, ct) AS
(
       SELECT 'AAHHYGAQCDKTN',
              (
                       SELECT   group_concat(letter,'')
                       FROM     suffix_array_sequence
                       WHERE    position>(SELECT num FROM suffix_array WHERE position=(SELECT max(position)/2 FROM suffix_array))
                       AND      position<((SELECT num FROM suffix_array WHERE position=(SELECT max(position)/2 FROM suffix_array))+length('AAHHYGAQCDKTN')+1)
                       ORDER BY position ASC),
              max(position)/2,
              max(position),
              0, 0
       FROM   suffix_array
       UNION ALL
       SELECT search_string,
              (
                       SELECT   group_concat(letter,'')
                       FROM     suffix_array_sequence
                       WHERE    position>(SELECT num FROM suffix_array WHERE position=pos)
                       AND      position<((SELECT num FROM suffix_array WHERE position=pos)+length(search_string)+1)
                       ORDER BY position ASC) AS compare_string,
              CASE
                     WHEN search_string < (
                       SELECT   group_concat(letter,'')
                       FROM     suffix_array_sequence
                       WHERE    position>(SELECT num FROM suffix_array WHERE position=pos)
                       AND      position<((SELECT num FROM suffix_array WHERE position=pos)+length(search_string)+1)
                       ORDER BY position ASC) THEN pos-((pos-lower_limit)/2)
                     WHEN search_string > (
                       SELECT   group_concat(letter,'')
                       FROM     suffix_array_sequence
                       WHERE    position>(SELECT num FROM suffix_array WHERE position=pos)
                       AND      position<((SELECT num FROM suffix_array WHERE position=pos)+length(search_string)+1)
                       ORDER BY position ASC) THEN pos+((upper_limit-pos)/2)
                     ELSE pos
              END pos,
              CASE
                     WHEN search_string < (
                       SELECT   group_concat(letter,'')
                       FROM     suffix_array_sequence
                       WHERE    position>(SELECT num FROM suffix_array WHERE position=pos)
                       AND      position<((SELECT num FROM suffix_array WHERE position=pos)+length(search_string)+1)
                       ORDER BY position ASC) THEN pos
                     WHEN search_string > (
                       SELECT   group_concat(letter,'')
                       FROM     suffix_array_sequence
                       WHERE    position>(SELECT num FROM suffix_array WHERE position=pos)
                       AND      position<((SELECT num FROM suffix_array WHERE position=pos)+length(search_string)+1)
                       ORDER BY position ASC) THEN upper_limit
                     ELSE upper_limit
              END upper_limit,
              CASE
                     WHEN search_string < (
                       SELECT   group_concat(letter,'')
                       FROM     suffix_array_sequence
                       WHERE    position>(SELECT num FROM suffix_array WHERE position=pos)
                       AND      position<((SELECT num FROM suffix_array WHERE position=pos)+length(search_string)+1)
                       ORDER BY position ASC) THEN lower_limit
                     WHEN search_string > (
                       SELECT   group_concat(letter,'')
                       FROM     suffix_array_sequence
                       WHERE    position>(SELECT num FROM suffix_array WHERE position=pos)
                       AND      position<((SELECT num FROM suffix_array WHERE position=pos)+length(search_string)+1)
                       ORDER BY position ASC) THEN pos
                     ELSE lower_limit
              END lower_limit,
  ct+1
       FROM   suffix_search
       WHERE  search_string != compare_string AND ct<28 )
SELECT *
FROM   suffix_search ORDER BY ct DESC;









