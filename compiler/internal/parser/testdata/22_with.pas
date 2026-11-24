program WithAnweisung;
type
  TDatum = record
    Tag, Monat, Jahr: Integer
  end;
  TPerson = record
    Name: string;
    Geburt: TDatum
  end;
  PPerson = ^TPerson;
var
  p: TPerson;
  q: PPerson;
  Liste: array[1..3] of TPerson;
  i: Integer;
begin
  with p do
  begin
    Name := 'Ada';
    with Geburt do
    begin
      Tag := 10; Monat := 12; Jahr := 1815
    end
  end;
  with p, Geburt do
    WriteLn(Name, ' ', Tag, '.', Monat, '.', Jahr);
  New(q);
  with q^, Geburt do Jahr := 1906;
  for i := 1 to 3 do
    with Liste[i].Geburt do
      Jahr := 2000 + i
end.
